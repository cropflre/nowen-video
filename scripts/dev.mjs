import { spawn, spawnSync } from 'node:child_process'
import { existsSync } from 'node:fs'
import net from 'node:net'
import path from 'node:path'
import process from 'node:process'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const scriptDir = path.dirname(__filename)
const projectRoot = path.resolve(scriptDir, '..')
const webRoot = path.join(projectRoot, 'web')

function parsePort(value, name) {
  const port = Number(value)
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error(`${name} 必须是 1-65535 之间的整数，当前值: ${value}`)
  }
  return port
}

function parseArguments(argv) {
  const result = {
    serverPort: process.env.SERVER_PORT ? parsePort(process.env.SERVER_PORT, 'SERVER_PORT') : 28888,
    webPort: process.env.WEB_PORT ? parsePort(process.env.WEB_PORT, 'WEB_PORT') : 28889,
  }

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index]
    if (arg === '--server-port') {
      result.serverPort = parsePort(argv[++index], '--server-port')
    } else if (arg.startsWith('--server-port=')) {
      result.serverPort = parsePort(arg.slice('--server-port='.length), '--server-port')
    } else if (arg === '--web-port') {
      result.webPort = parsePort(argv[++index], '--web-port')
    } else if (arg.startsWith('--web-port=')) {
      result.webPort = parsePort(arg.slice('--web-port='.length), '--web-port')
    } else {
      throw new Error(`未知参数: ${arg}`)
    }
  }

  return result
}

function canBind(port, host, ipv6Only = false) {
  return new Promise((resolve) => {
    const server = net.createServer()
    server.unref()
    server.once('error', (error) => {
      // 某些系统完全禁用 IPv6，此时只要 IPv4 可绑定即可继续。
      if (host === '::' && ['EAFNOSUPPORT', 'EADDRNOTAVAIL'].includes(error.code)) {
        resolve(true)
        return
      }
      resolve(false)
    })
    server.listen({ port, host, ipv6Only, exclusive: true }, () => {
      server.close(() => resolve(true))
    })
  })
}

async function canListen(port) {
  if (!(await canBind(port, '0.0.0.0'))) return false
  return canBind(port, '::', true)
}

async function findFreePort(preferredPort, excludedPorts = new Set(), maxAttempts = 200) {
  for (let offset = 0; offset < maxAttempts; offset += 1) {
    const candidate = preferredPort + offset
    if (candidate > 65535) break
    if (excludedPorts.has(candidate)) continue
    if (await canListen(candidate)) return candidate
  }
  throw new Error(`从端口 ${preferredPort} 开始连续检查 ${maxAttempts} 个端口后，仍未找到可用端口。`)
}

function resolveVersion() {
  for (const candidate of [process.env.NOWEN_VERSION, process.env.VITE_APP_VERSION]) {
    if (candidate?.trim()) return candidate.trim().replace(/^v/, '')
  }

  const result = spawnSync(
    'git',
    ['describe', '--tags', '--abbrev=0', '--match', 'v[0-9]*'],
    { cwd: projectRoot, encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] },
  )
  if (result.status === 0 && result.stdout.trim()) {
    return result.stdout.trim().replace(/^v/, '')
  }
  return '0.1.0'
}

function resolveNpmInvocation(args) {
  // npm run 会提供 npm_execpath。直接用当前 node 执行 npm-cli.js，
  // 避免 Windows 新版 Node 对 npm.cmd 直接 spawn 时抛出 EINVAL。
  const npmExecPath = process.env.npm_execpath
  if (npmExecPath && existsSync(npmExecPath)) {
    return {
      command: process.execPath,
      args: [npmExecPath, ...args],
    }
  }

  if (process.platform === 'win32') {
    return {
      command: process.env.ComSpec || 'cmd.exe',
      args: ['/d', '/s', '/c', `npm ${args.join(' ')}`],
    }
  }

  return { command: 'npm', args }
}

function runNpmSync(args, options) {
  const invocation = resolveNpmInvocation(args)
  return spawnSync(invocation.command, invocation.args, options)
}

function installWebDependencies() {
  if (existsSync(path.join(webRoot, 'node_modules'))) return

  console.log('[准备] 未检测到 web/node_modules，正在安装前端依赖...')
  const result = runNpmSync(['install'], {
    cwd: webRoot,
    stdio: 'inherit',
    env: process.env,
  })
  if (result.error) {
    throw new Error(`前端依赖安装失败: ${result.error.message}`)
  }
  if (result.status !== 0) {
    throw new Error(`前端依赖安装失败，退出码: ${result.status ?? 'unknown'}`)
  }
}

function startProcess(command, args, options) {
  return spawn(command, args, {
    ...options,
    stdio: 'inherit',
    detached: process.platform !== 'win32',
  })
}

function startNpm(args, options) {
  const invocation = resolveNpmInvocation(args)
  return startProcess(invocation.command, invocation.args, options)
}

function stopProcess(child) {
  if (!child || child.exitCode !== null || child.killed) return

  if (process.platform === 'win32') {
    spawnSync('taskkill', ['/PID', String(child.pid), '/T', '/F'], {
      stdio: 'ignore',
      windowsHide: true,
    })
    return
  }

  try {
    process.kill(-child.pid, 'SIGTERM')
  } catch {
    try {
      child.kill('SIGTERM')
    } catch {
      // 进程已退出。
    }
  }
}

async function main() {
  const requested = parseArguments(process.argv.slice(2))
  const serverPort = await findFreePort(requested.serverPort)
  const webPort = await findFreePort(requested.webPort, new Set([serverPort]))
  const version = resolveVersion()

  if (serverPort !== requested.serverPort) {
    console.warn(`[端口] 后端优先端口 ${requested.serverPort} 已占用，自动切换到 ${serverPort}`)
  }
  if (webPort !== requested.webPort) {
    console.warn(`[端口] 前端优先端口 ${requested.webPort} 已占用，自动切换到 ${webPort}`)
  }

  installWebDependencies()

  console.log('')
  console.log('============================================================')
  console.log(' nowen-video 本地开发环境（Server Lite）')
  console.log(` 后端地址: http://localhost:${serverPort}`)
  console.log(` 前端地址: http://localhost:${webPort}`)
  console.log(` 前端代理: http://localhost:${serverPort}`)
  console.log(' 停止服务: Ctrl+C')
  console.log('============================================================')
  console.log('')

  const sharedEnv = {
    ...process.env,
    NOWEN_VERSION: version,
    VITE_APP_VERSION: version,
  }

  let server = null
  let web = null
  let shuttingDown = false

  const shutdown = (exitCode = 0) => {
    if (shuttingDown) return
    shuttingDown = true
    stopProcess(web)
    stopProcess(server)
    process.exitCode = exitCode
  }

  try {
    server = startProcess('go', ['run', './cmd/server-lite'], {
      cwd: projectRoot,
      env: {
        ...sharedEnv,
        CGO_ENABLED: '1',
        NOWEN_DEBUG: process.env.NOWEN_DEBUG || 'true',
        NOWEN_APP_PORT: String(serverPort),
        SERVER_PORT: String(serverPort),
      },
    })

    web = startNpm(
      ['run', 'dev', '--', '--port', String(webPort), '--host', '--strictPort'],
      {
        cwd: webRoot,
        env: {
          ...sharedEnv,
          WEB_PORT: String(webPort),
          SERVER_PORT: String(serverPort),
          VITE_API_PROXY_TARGET: `http://localhost:${serverPort}`,
        },
      },
    )
  } catch (error) {
    // spawn 可能同步抛错。确保前端启动失败时不会遗留已经运行的 Go 后端。
    stopProcess(web)
    stopProcess(server)
    throw error
  }

  process.on('SIGINT', () => shutdown(0))
  process.on('SIGTERM', () => shutdown(0))

  server.on('error', (error) => {
    console.error(`[后端] 启动失败: ${error.message}`)
    shutdown(1)
  })
  web.on('error', (error) => {
    console.error(`[前端] 启动失败: ${error.message}`)
    shutdown(1)
  })

  server.on('exit', (code, signal) => {
    if (!shuttingDown) {
      console.error(`[后端] 已退出，code=${code ?? 'null'} signal=${signal ?? 'null'}`)
      shutdown(code ?? 1)
    }
  })
  web.on('exit', (code, signal) => {
    if (!shuttingDown) {
      console.error(`[前端] 已退出，code=${code ?? 'null'} signal=${signal ?? 'null'}`)
      shutdown(code ?? 1)
    }
  })
}

main().catch((error) => {
  console.error(`[dev] ${error instanceof Error ? error.message : String(error)}`)
  process.exitCode = 1
})

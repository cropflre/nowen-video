#!/usr/bin/env node
import { execFileSync } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import { homedir } from 'node:os'
import { join, resolve } from 'node:path'

const repository = process.argv[2] || 'cropflre/nowen-video'
const dockerConfigDir = process.env.DOCKER_CONFIG ? resolve(process.env.DOCKER_CONFIG) : join(homedir(), '.docker')
const configPath = join(dockerConfigDir, 'config.json')
const dockerHubServers = [
  'https://index.docker.io/v1/',
  'https://index.docker.io/v1',
  'registry-1.docker.io',
  'docker.io',
]

function die(message) {
  console.error(`[docker-auth] ${message}`)
  process.exit(1)
}

function decodeInlineAuth(value) {
  if (!value) return null
  try {
    const decoded = Buffer.from(value, 'base64').toString('utf8')
    const separator = decoded.indexOf(':')
    if (separator <= 0) return null
    return { username: decoded.slice(0, separator), secret: decoded.slice(separator + 1) }
  } catch {
    return null
  }
}

function helperCredential(helper, server) {
  if (!helper) return null
  const executable = `docker-credential-${helper}`
  try {
    const stdout = execFileSync(executable, ['get'], {
      input: `${server}\n`,
      encoding: 'utf8',
      stdio: ['pipe', 'pipe', 'ignore'],
      timeout: 10_000,
    })
    const payload = JSON.parse(stdout)
    if (!payload.Username || !payload.Secret) return null
    return { username: payload.Username, secret: payload.Secret }
  } catch {
    return null
  }
}

function loadCredential(config) {
  for (const server of dockerHubServers) {
    const inline = decodeInlineAuth(config.auths?.[server]?.auth)
    if (inline) return { ...inline, server, source: 'config auth' }
  }

  for (const server of dockerHubServers) {
    const helper = config.credHelpers?.[server]
    const credential = helperCredential(helper, server)
    if (credential) return { ...credential, server, source: `credential helper ${helper}` }
  }

  if (config.credsStore) {
    for (const server of dockerHubServers) {
      const credential = helperCredential(config.credsStore, server)
      if (credential) return { ...credential, server, source: `credential store ${config.credsStore}` }
    }
  }
  return null
}

function decodeJwtPayload(token) {
  const parts = String(token || '').split('.')
  if (parts.length < 2) return null
  try {
    return JSON.parse(Buffer.from(parts[1], 'base64url').toString('utf8'))
  } catch {
    return null
  }
}

if (!existsSync(configPath)) {
  die(`未找到 Docker 登录配置 ${configPath}。请先执行 docker login。`)
}

let config
try {
  config = JSON.parse(readFileSync(configPath, 'utf8'))
} catch (error) {
  die(`无法读取 Docker 配置: ${error.message}`)
}

const credential = loadCredential(config)
if (!credential) {
  die('没有找到 Docker Hub 登录凭据。请先执行 docker login。')
}

const tokenUrl = new URL('https://auth.docker.io/token')
tokenUrl.searchParams.set('service', 'registry.docker.io')
tokenUrl.searchParams.set('scope', `repository:${repository}:pull,push`)

let response
try {
  response = await fetch(tokenUrl, {
    headers: {
      authorization: `Basic ${Buffer.from(`${credential.username}:${credential.secret}`).toString('base64')}`,
      'user-agent': 'nowen-video-release/dockerhub-auth-preflight',
    },
    signal: AbortSignal.timeout(20_000),
  })
} catch (error) {
  die(`无法连接 Docker Hub 认证服务: ${error.message}`)
}

if (!response.ok) {
  die(`Docker Hub 登录凭据无效或认证失败: HTTP ${response.status}。请重新执行 docker login。`)
}

const body = await response.json().catch(() => ({}))
const token = body.token || body.access_token
if (!token) die('Docker Hub 未返回访问 token，无法验证 push 权限。')

const payload = decodeJwtPayload(token)
if (!payload) die('无法解析 Docker Hub 授权 token，不能确认 push 权限；请重新 docker login 后再试。')

const access = Array.isArray(payload.access) ? payload.access : []
const grant = access.find((entry) => entry?.type === 'repository' && entry?.name === repository)
const actions = Array.isArray(grant?.actions) ? grant.actions : []
if (!actions.includes('push')) {
  die(`当前 Docker Hub 账号 ${credential.username} 没有 ${repository} 的 push 权限。`)
}

console.log(`[docker-auth] Docker Hub 登录有效: ${credential.username}`)
console.log(`[docker-auth] push 权限有效: ${repository}`)
console.log(`[docker-auth] credential source: ${credential.source}`)

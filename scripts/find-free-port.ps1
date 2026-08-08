[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateRange(1, 65535)]
    [int]$PreferredPort,

    [ValidateRange(1, 5000)]
    [int]$MaxAttempts = 200,

    [int[]]$ExcludePort = @()
)

$ErrorActionPreference = 'Stop'

function Test-TcpPortAvailable {
    param(
        [Parameter(Mandatory = $true)]
        [int]$Port
    )

    # 先读取当前监听端口，再实际尝试绑定，避免只依赖 netstat 文本解析。
    $ipProperties = [System.Net.NetworkInformation.IPGlobalProperties]::GetIPGlobalProperties()
    $activePorts = $ipProperties.GetActiveTcpListeners() | ForEach-Object { $_.Port }

    if ($activePorts -contains $Port) {
        return $false
    }

    $listener = [System.Net.Sockets.TcpListener]::new(
        [System.Net.IPAddress]::Any,
        $Port
    )

    try {
        $listener.Start()
        return $true
    }
    catch {
        return $false
    }
    finally {
        try {
            $listener.Stop()
        }
        catch {
            # 未成功启动时 Stop 可能抛错，不影响端口探测结果。
        }
    }
}

for ($offset = 0; $offset -lt $MaxAttempts; $offset++) {
    $candidate = $PreferredPort + $offset
    if ($candidate -gt 65535) {
        break
    }

    if ($ExcludePort -contains $candidate) {
        continue
    }

    if (Test-TcpPortAvailable -Port $candidate) {
        [Console]::Out.WriteLine($candidate)
        exit 0
    }
}

Write-Error "从端口 $PreferredPort 开始连续检查 $MaxAttempts 个端口后，仍未找到可用 TCP 端口。"
exit 1

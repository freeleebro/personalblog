# 本机一键发布：打包 → 上传 → 在服务器上执行 deploy.sh（含自动回滚）。
#
# 用法（在仓库根目录）：
#   pwsh scripts/release.ps1
#   pwsh scripts/release.ps1 -SshHost leebro
#
# 前置条件：本机已配置到服务器的 SSH 免密登录；服务器上 Docker 可用。

param(
  [string]$SshHost = 'leebro',
  [string]$RemoteTarball = '/opt/beichuang-upload.tar.gz'
)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$tarPath = Join-Path (Split-Path -Parent $repo) 'beichuang-deploy.tar.gz'

Write-Host "==> 打包 $repo"
if (Test-Path $tarPath) { Remove-Item $tarPath -Force }
$tarArgs = @('-czf', $tarPath, '--exclude=*.exe')
$excludes = @(
  'node_modules', '.next', '.git', '.devdb',
  'backend/data', 'backend/tmp', '.env', '.env.server',
  '.workbuddy', '.agents', '.taste-skill'
)
foreach ($e in $excludes) { $tarArgs += "--exclude=$e" }
$tarArgs += '.'
Push-Location $repo
try {
  & tar @tarArgs
  if ($LASTEXITCODE -ne 0) { throw "tar 打包失败（exit $LASTEXITCODE）" }
} finally {
  Pop-Location
}
Write-Host ("    包大小: {0:N0} 字节" -f (Get-Item $tarPath).Length)

Write-Host "==> 上传源码包与部署脚本"
& scp -q $tarPath "${SshHost}:${RemoteTarball}"
if ($LASTEXITCODE -ne 0) { throw "上传源码包失败" }
& scp -q (Join-Path $repo 'scripts/deploy.sh') "${SshHost}:/opt/beichuang-deploy.sh"
if ($LASTEXITCODE -ne 0) { throw "上传部署脚本失败" }

Write-Host "==> 在服务器上执行部署（失败会自动回滚）"
& ssh $SshHost "chmod +x /opt/beichuang-deploy.sh && bash /opt/beichuang-deploy.sh ${RemoteTarball}"
if ($LASTEXITCODE -ne 0) {
  throw "部署失败，查看服务器日志：ssh $SshHost 'tail -n 80 /var/log/beichuang-deploy.log'"
}

Write-Host "==> 完成。线上验证：curl -sI http://127.0.0.1/ | head -1"
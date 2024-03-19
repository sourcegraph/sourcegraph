# Windows support

> NOTE: Windows support is still rough around the edges. If you encounter issues, please let us know by [filing an issue](https://github.com/sourcegraph/src-cli/issues/new).

## Installation

### Install via PowerShell

Run in PowerShell as administrator:

```powershell
New-Item -ItemType Directory 'C:\Program Files\Sourcegraph'

Invoke-WebRequest https://sourcegraph.com/.api/src-cli/src_windows_amd64.exe -OutFile 'C:\Program Files\Sourcegraph\src.exe'

[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', [EnvironmentVariableTarget]::Machine) + ';C:\Program Files\Sourcegraph', [EnvironmentVariableTarget]::Machine)
$env:Path += ';C:\Program Files\Sourcegraph'
```

### Install manually

1. Download the latest [src_windows_amd64.exe](https://sourcegraph.com/.api/src-cli/src_windows_amd64.exe)
2. Place the file under e.g. `C:\Program Files\Sourcegraph\src.exe`
3. Add that directory to your system PATH to access it from any command prompt

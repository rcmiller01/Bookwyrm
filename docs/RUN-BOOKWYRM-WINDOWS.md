# Run Bookwyrm on Windows (ZIP)

1. Extract `bookwyrm-<version>-windows.zip` to `C:\ProgramData` (this creates `C:\ProgramData\Bookwyrm`).
2. Edit `config\bookwyrm.env`.
3. Edit `config\metadata-service.yaml`.
4. Run:

```powershell
cd C:\ProgramData\Bookwyrm
.\bin\bookwyrm-launcher.exe run --base-dir C:\ProgramData\Bookwyrm
```

5. Open `http://localhost:8090`.

Alternative:

- Right-click `scripts\start-bookwyrm.ps1`
- Select `Run with PowerShell`

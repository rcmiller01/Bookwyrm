# Windows Zip Distribution

Bookwyrm alpha distributions are zip-only to avoid unsigned installer confusion.

## Artifact

- `bookwyrm-<version>-windows.zip`

## Install from zip

1. Extract zip to a stable folder, for example `C:\ProgramData\Bookwyrm`.
2. Ensure these subfolders exist after extraction:
   - `bin\`
   - `config\`
   - `logs\`
   - `data\`
3. Edit `config\bookwyrm.env` and `config\metadata-service.yaml` for your environment.
4. Start Bookwyrm:

```powershell
cd C:\ProgramData\Bookwyrm
.\bin\bookwyrm-launcher.exe run --base-dir C:\ProgramData\Bookwyrm
```

5. Open `http://localhost:8090` and complete setup.

## Upgrade behavior

- Replace `bin\` with files from the new zip.
- Keep `config\`, `data\`, and `logs\`.
- Do not overwrite secrets unless intentionally rotating values.

## Post-install validation

```powershell
Invoke-RestMethod http://localhost:8090/api/v1/system/dependencies
Invoke-RestMethod http://localhost:8090/api/v1/system/migration-status
```

- `can_function_now=true` confirms startup success.
- Use [alpha-validation.md](alpha-validation.md) to record clean-machine results per release tag.

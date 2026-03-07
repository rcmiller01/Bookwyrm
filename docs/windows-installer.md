# Windows Installer (Phase 21 Target)

This document defines the intended installer behavior for native Windows packaging.

## Installer responsibilities

- Place binaries under `C:\ProgramData\Bookwyrm\bin\`
- Place writable config under `C:\ProgramData\Bookwyrm\config\`
- Place logs under `C:\ProgramData\Bookwyrm\logs\`
- Optionally register Bookwyrm as a Windows service
- Optionally open `http://localhost:8090` after install

## Upgrade behavior

- Replace only `bin\`
- Preserve `config\` and `data\`
- Never overwrite secrets silently

## Uninstall behavior

- Remove binaries
- Prompt whether to keep `config\`, `logs\`, and data


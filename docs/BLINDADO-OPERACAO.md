# BLINDADO - Operacao MultiSend

## Check rápido
- Confirme que o Agent está instalado em `C:\Program Files\MultiSend\bin`.
- Confirme que `multisend-agent.exe` responde na API local.
- Use um arquivo real como argumento.

## Run command
```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Program Files\MultiSend\bin\multisend-launcher.ps1" "C:\caminho\do\arquivo.ext"
```

## Leitura de resultado
- `success`: aparece `Transfer started. Job ID: ...` e depois `Transfer completed successfully.`
- `peer-not-found`: aparece `No MultiSend peers found in agent cache.` e depois `Canceled.` ou falha ao enviar sem `job_id`.
- `unreachable-network`: aparece `Failed to start transfer: ...`, `Transfer failed: ...`, ou timeout com `Transfer status timeout.` quando o peer existe mas não alcança a rede.
- `api-offline`: aparece `MultiSend Agent não está respondendo na API local (56221-56230)...` ou `MultiSend Agent is not responding on local API range after start attempt.`

## Regra de leitura
- `success` = envio concluído.
- `peer-not-found` = agente vivo, mas sem peers em cache.
- `unreachable-network` = destino informado, mas sem conexão útil com a rede.
- `api-offline` = API local do Agent fora do ar.

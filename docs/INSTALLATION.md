# Instalação, upgrade e reparo

## Caminho recomendado

Execute `dist\MultiSendSetup.exe` com UAC. O setup instala o agente e os launchers em
`C:\Program Files\MultiSend\bin`, registra menus/atalhos e chama o script de instalação com uma
escolha explícita de autostart.

As regras de entrada criadas são:

| Regra | Protocolo/porta | Perfis | Origem |
|---|---|---|---|
| MultiSend Agent TCP 56200-56210 | TCP 56200–56210 | Domain, Private | LocalSubnet |
| MultiSend Discovery UDP 56211-56220 | UDP 56211–56220 | Domain, Private | LocalSubnet |
| MultiSend Control TCP 56231-56240 | TCP 56231–56240 | Domain, Private | LocalSubnet |

Todas apontam para o `multisend-agent.exe` instalado. O perfil público e endereços remotos globais
não são habilitados.

## Script administrativo

`install-multisend-node-v5.ps1` aceita exatamente um modo:

- `-Install`: instala ou atualiza a partir do payload ao lado do script;
- `-Repair`: reaplica arquivos, configuração administrativa, firewall, menus e atalhos; se o agente
  estava ativo antes do reparo, ele é iniciado novamente ao final;
- `-Uninstall`: remove itens gerenciados;
- `-Test`: valida arquivos e API; elevado, também valida todos os filtros do firewall;
- `-Doctor`: executa o diagnóstico do agente;
- `-LabSmoke`: executa o smoke LAN documentado em `SMOKE_TESTS.md`.

Opções relevantes:

- `-StartAgentNow`: inicia o agente após instalar;
- `-EnableAutoStart` / `-DisableAutoStart`: escolhas explícitas e mutuamente exclusivas;
- quando nenhuma delas é passada em um upgrade/reparo manual, o estado atual de autostart é
  preservado;
- `-ReceiveRoot` e `-DisplayName` sobrescrevem somente esses valores.

O reparo preserva os campos existentes de `%APPDATA%\MultiSend\config.json`; o agente faz a
migração de schema e mantém valores booleanos explicitamente desativados. Configuração inválida é
copiada para um arquivo `.invalid-<data>` antes da substituição.

## Arquivos preservados

O uninstall mantém:

- `%APPDATA%\MultiSend` — configuração e histórico local;
- `C:\ProgramData\MultiSend` — logs e estado de instalação;
- `receive_path` — arquivos recebidos e downloads.

Reinstalar ou reparar não apaga esses dados.

## Validação e rollback

`rebuild-multisend-setup.ps1` executa testes, compila os três binários, sincroniza o payload e gera
o setup. O executável anterior é copiado para `dist\Old` antes da substituição.

Após upgrade, execute `-Test` elevado. O gate compara SHA-256 entre payload e instalação, valida
os filtros completos do firewall e consulta `/health`. Para restaurar arquivos corrompidos, use o
setup ou um script acompanhado do payload completo; o script já instalado só consegue reaplicar os
arquivos que ainda existem ao lado dele.

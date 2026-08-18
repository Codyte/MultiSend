# Reformulação da interface do MultiSend

## Objetivo

Substituir gradualmente as telas WinForms escritas em PowerShell por uma interface web flexível,
sem reescrever o motor de transferência e sem acoplar a vida útil das transferências à janela da
aplicação.

O agente Go continua sendo a fonte de verdade para descoberta, envios, downloads, retomada,
telemetria e configuração. A interface é apenas um cliente local.

## Progresso

Implementado em 2026-08-18:

- painel responsivo incorporado em `/ui/`;
- resumo do agente, peers, interfaces, downloads, envios e pulls;
- início de download HTTP(S), envio LAN e recebimento remoto;
- ações de interromper, retomar e remover histórico terminal com confirmação;
- abertura pelo launcher e pelo atalho principal do instalador;
- proteção de `Host` e `Origin`, CSP, limites de corpo e timeouts HTTP;
- shutdown por sinal e endpoint local de encerramento controlado;
- gravação atômica da configuração;
- ordenação estável das coleções em memória.
- API `/api/v1/config` com DTO explícito, validação de caminhos e segredo sempre redigido;
- tela web de configurações com identidade, interfaces, limpeza, pipeline e segurança;
- atalho de configurações migrado para `/ui/?view=settings`.
- histórico de downloads HTTP(S) reidratado de manifests válidos ao iniciar o agente;
- recuperação segura de chunks interrompidos e retenção de até 500 jobs terminais em memória.
- sidecars locais versionados para restaurar envios LAN e pulls com seu contexto operacional;
- retomada P2P habilitada somente quando sidecar, manifesto e arquivo-fonte continuam coerentes.
- atalhos de download, recebimento, configurações e protocolo `multisend://` direcionados à UI web;
- instalador preservando a configuração existente durante reparo/upgrade.
- `multisend-download-ui.ps1` reduzido a shim de compatibilidade para o launcher/web;
- ZIPs de seleção múltipla pertencem ao job e só são removidos após sucesso ou exclusão explícita
  do histórico, preservando a possibilidade de retomada.

Os smokes de envio e download agora usam diretórios exclusivos, validam retomada/finalização e
removem seus próprios artefatos. A próxima mudança arquitetural relevante é medir o polling antes
de considerar atualização em tempo real; ela não é necessária para o volume atual.

## Decisões

1. **Agente independente.** Fechar a interface não pode cancelar transferências.
2. **Uma única interface web.** HTML, CSS e JavaScript são incorporados ao executável do agente e
   servidos apenas pela API loopback.
3. **Sem framework inicialmente.** A primeira versão usa APIs nativas do navegador e a biblioteca
   padrão do Go. Um framework só será introduzido se a complexidade medida justificar o custo.
4. **Compatibilidade durante a migração.** Os endpoints e scripts PowerShell existentes continuam
   funcionando até a nova interface atingir paridade.
5. **Integração nativa separada.** Explorer, seletor de arquivos e arrastar/soltar caminhos locais
   permanecem no launcher atual. Uma janela Wails poderá reutilizar o frontend quando essa etapa
   for necessária.
6. **Segredos pertencem ao agente.** A futura API de configuração nunca deve retornar o segredo do
   nó por padrão.

## Estado encontrado

- O antigo `multisend-download-ui.ps1`, com mais de 1.300 linhas de layout e polling, foi substituído
  por um wrapper curto; painel e atualização de estado pertencem à UI incorporada.
- O botão de pausa usa o endpoint de cancelamento; o produto ainda não diferencia pausa real de
  cancelamento retomável.
- `multisend-settings-ui.ps1` permanece apenas como fallback para operações ainda não expostas na
  tela web, como administração explícita do segredo. A configuração cotidiana usa a API Go.
- A porta local é selecionada automaticamente dentro da faixa configurada; o campo legado passou a
  ser somente leitura até a migração da configuração para a API.
- Downloads HTTP(S), jobs de envio e pulls possuem histórico local. Operações interrompidas nunca
  voltam como ativas; a retomada depende da validação do contrato persistido de cada tipo.
- As APIs necessárias para o primeiro painel já existem: `/health`, `/peers`, `/send`, `/jobs`,
  `/pulls`, `/downloads`, `/receiver/sessions` e `/interfaces`.

## Arquitetura-alvo

```text
Explorer / protocolo / extensão
              |
              v
      launcher ou shell nativo --------+
                                        |
navegador ou janela Wails -> UI web -> API local -> agente Go
                                                   |-- transferências LAN
                                                   |-- downloads HTTP(S)
                                                   |-- descoberta
                                                   `-- configuração e histórico
```

No primeiro estágio, o agente serve `/ui/` e a própria interface usa URLs relativas para acessar a
API no mesmo origin. Isso evita CORS e mantém o serviço restrito ao loopback.

## Contratos de segurança

- A API local escuta somente em `127.0.0.1`.
- Requisições com `Host` não local são rejeitadas.
- Quando o navegador envia `Origin`, ele deve ser exatamente o mesmo origin da API.
- Respostas da interface usam CSP, `nosniff` e política de referrer restritiva.
- Corpos JSON têm limite explícito.
- Endpoints legados toleram campos adicionais enquanto os clientes PowerShell coexistirem; a futura
  API `/api/v1` poderá adotar decodificação estrita.
- A API de controle de rede mantém autenticação e allowlist próprias.
- A futura configuração web deve separar leitura, atualização, regeneração e exportação de segredo.

Loopback não substitui autorização: qualquer endpoint futuro que abra arquivos, revele segredos ou
execute ações administrativas deverá exigir uma intenção local específica e auditável.

## Etapas

### 1. Fundação e painel inicial

- Incorporar os assets web ao agente.
- Expor `/ui/` e redirecionar `/` para a interface.
- Mostrar saúde do agente, peers, interfaces, downloads, envios e pulls.
- Permitir iniciar download, cancelar e retomar operações.
- Manter polling de um segundo, atualizando somente os elementos necessários.
- Adicionar shutdown por sinal, timeouts HTTP, limites JSON e ordenação estável.
- Encerrar e reiniciar o agente pelo endpoint local controlado, mantendo `Stop-Process` apenas como
  fallback de compatibilidade.

### 2. Configurações

- Criar uma API versionada de configuração com DTO editável explícito.
- Validar no Go antes de persistir.
- Gravar configuração de forma atômica.
- Informar campos aplicados ao vivo e campos que exigem reinício.
- Corrigir o contrato entre porta fixa e faixa de portas.

Status: API e tela web implementadas. Exportação/revelação do segredo permanece no fallback nativo;
portas de runtime continuam somente para leitura.

### 3. Histórico e recuperação

- Definir retenção de jobs concluídos.
- Reidratar downloads e transferências a partir dos manifests válidos.
- Remover itens somente por ação explícita ou política configurada.
- Preservar ordenação estável por atualização mais recente.

Status: downloads HTTP(S) implementados com retenção de 500 jobs terminais, varredura limitada e
validação estrita de raiz, ID, URL, plano de chunks e caminho da sessão. Envios LAN e pulls usam
sidecars locais versionados, sem alterar o protocolo de rede. Itens terminais podem ser removidos
explicitamente, preservando os arquivos publicados. Detalhes em `DOWNLOAD_HISTORY.md` e
`OPERATION_HISTORY.md`.

### 4. Integração desktop

- Manter o diálogo nativo de envio pelo Explorer como fallback para seleção múltipla e criação de
  ZIP; envio unitário por caminho absoluto já está disponível na interface web.
- Avaliar Wails para janela, diálogo nativo e drag-and-drop com caminhos absolutos.
- Manter o agente em processo separado mesmo quando houver uma janela Wails.
- Remover as telas PowerShell apenas após testes de paridade e atualização do instalador.

### 5. Atualização em tempo real

Polling é suficiente para a primeira versão. Server-Sent Events só deve ser adicionado se métricas
mostrarem atraso, consumo ou churn visual relevantes. WebSocket não é necessário para o contrato
atual.

## Organização pretendida

```text
cmd/multisend-agent/
  http_api.go       roteadores, limites e middleware HTTP
  web.go            assets incorporados e handler da interface
  web/
    index.html
    styles.css
    app.js
docs/
  UI_REFORMULATION.md
```

O `main.go` será dividido em arquivos do mesmo pacote conforme cada área for modificada. Não será
criada uma camada abstrata ou pacote novo apenas para reduzir o tamanho do arquivo.

## Critérios de aceite

- `go test ./...` e `go test -race ./...` passam.
- Os três binários Go compilam.
- `/` redireciona para `/ui/` e os assets são entregues com os headers de segurança.
- O painel cobre loading, vazio, erro, sucesso e estados responsivos.
- Clientes PowerShell existentes continuam consumindo os endpoints legados.
- Requisições locais válidas funcionam; `Host` ou `Origin` externos são rejeitados.
- Fechar o navegador não afeta o agente nem jobs ativos.

## Itens deliberadamente adiados

- Framework frontend, biblioteca de componentes e store global.
- SSE/WebSocket.
- Empacotamento Wails.
- Remoção do WinForms de configurações após existir um fluxo nativo explícito para pareamento,
  exportação e regeneração de segredo.
- Banco de dados exclusivo para histórico da UI.

Esses itens serão adicionados somente quando houver necessidade funcional ou evidência de que a
solução nativa atingiu seu limite.

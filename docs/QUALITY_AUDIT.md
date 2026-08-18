# Auditoria de qualidade

Linha de base medida em 2026-08-18 com `go test -cover ./...`. Cobertura é usada como indicação de
contratos sem prova, não como meta isolada.

## Correções concluídas nesta auditoria

- faixas inválidas de porta falham antes de iniciar a varredura;
- seleção de porta foi exercitada com sockets TCP e UDP reais em loopback;
- descoberta ignora anúncios com endereço ou porta inválidos;
- ordenação de peers usa nome e `node_id`, eliminando desempate variável de map;
- expiração de peers e supressão de probes duplicados possuem testes determinísticos;
- detecção de interfaces valida IPv4, exclusão de loopback/link-local e deduplicação;
- CLIs divididos agora usam identidade comum, offsets, SHA-256, ACK verificado e manifesto
  serializado;
- receptor CLI rejeita traversal, limites incoerentes, hash inválido e autenticação incorreta.
- staging ZIP do launcher passou a pertencer ao job; não é mais apagado enquanto a transferência
  assíncrona ainda pode estar lendo ou retomando o arquivo;
- o WinForms legado de downloads foi substituído por um shim compatível para a UI web.
- o pacote experimental `internal/memory`, sem consumidores, e sua dependência SQLite/CGO foram
  removidos; os binários do produto agora dependem somente da biblioteca padrão do Go.
- pulls autenticados agora propagam Bearer em todas as chamadas de controle e falham fechados sem
  segredo; respostas remotas são limitadas a 1 MiB;
- a whitelist de `remote_send_roots` usa caminhos resolvidos, bloqueando escape por symlink ou
  junction em um diretório pai;
- downloads concorrentes com o mesmo nome recebem destinos distintos, e o merge publica o arquivo
  final somente depois de validar o temporário completo;
- criação de ZIP e staging remoto reporta falhas de fechamento e limpa preparações que não chegaram
  a criar um job.
- o modo `-Repair` preserva o estado operacional: um agente previamente ativo volta a iniciar após
  a substituição dos binários.
- enviar, receber e baixar foram reunidos em um formulário orientado pela origem, mantendo os três
  endpoints especializados; a UI não exige mais alternar entre abas operacionais;
- polling do painel passou de seis requisições a cada 1,2 s em repouso (aproximadamente 300/min)
  para um snapshot a cada 5 s (até 12/min); durante operações, o snapshot a cada 1,2 s limita o
  teto a 50/min, e consultas são suspensas com a página oculta ou nas configurações.
- remetente e receptores compartilham a validação estrutural do cabeçalho de chunk, eliminando
  regras divergentes de identidade, índices, offsets, tamanhos e SHA-256;
- chunks recebidos são gravados em temporário, sincronizados, verificados e publicados
  atomicamente; um chunk existente só é reutilizado quando tamanho e hash conferem;
- manifesto corrompido é preservado e conflito de identidade, destino ou plano de chunks é
  recusado, em vez de reiniciar silenciosamente uma sessão incompatível;
- destinos de recebimento resolvem symlinks e junctions nos ancestrais existentes antes de validar
  `receive_path`; ZIPs de pasta recusam links simbólicos explicitamente para evitar transferências
  incompletas;
- `runtime.json`, sidecars e históricos usam a mesma gravação JSON atômica; backups de configuração
  potencialmente sensíveis usam permissão `0600`;
- `cmd/multisend-agent/main.go` caiu de 2.824 para cerca de 526 linhas, com recepção, envio, pull e
  download separados em arquivos do mesmo pacote e sem introduzir abstrações ou dependências.
- `-Doctor`, `-LabSmoke` e `-DownloadSmoke` usam o mesmo encaminhamento no script administrativo;
  o smoke de download deixou de exigir chamada direta e divergente ao executável.
- uma seleção única do Explorer reutiliza o formulário web com origem e modo explícitos; o diálogo
  WinForms permanece somente para seleção múltipla ou falha ao abrir o navegador.

## Cobertura atual dos módulos auditados

| Pacote | Antes | Depois |
|---|---:|---:|
| `cmd/multirecv` | 0,0% | 55,0% |
| `cmd/multisend` | 0,0% | 18,6% |
| `internal/discovery` | 0,0% | 19,4% |
| `internal/netif` | 0,0% | 90,5% |
| `internal/ports` | 0,0% | 89,5% |

`discovery` permanece com cobertura percentual menor porque loops multicast, broadcast e ARP
dependem do sistema operacional e da rede. Os contratos puros e o processamento de anúncios estão
cobertos; testes de duas máquinas continuam no checklist de publicação.

## Validação obrigatória

```powershell
go test ./...
go vet ./...
go build ./...
```

O race detector ainda exige um compilador C e `CGO_ENABLED=1` nesta máquina. A ausência desse
toolchain deve ser reportada; nunca substituir o teste por uma alegação de aprovação.

## Próximas oportunidades baseadas em evidência

1. Executar LAN send/pull entre duas máquinas, incluindo arquivo e pasta zip/extract.
2. Rodar `go test -race ./...` quando houver toolchain CGO local.
3. Testar instalação e desinstalação em uma máquina Windows sem ambiente de desenvolvimento.

Framework frontend, banco de dados e nova abstração de rede permanecem adiados até existir uma
necessidade mensurável.

## Decisões e validações externas pendentes

- definir a experiência nativa que substituirá o último fallback WinForms de configurações para
  copiar, regenerar, importar e exportar o segredo do nó;
- autorizar a instalação de um compilador C local caso `go test -race ./...` deva fazer parte da
  validação nesta máquina;
- disponibilizar uma segunda máquina Windows para os testes LAN e uma máquina limpa para validar
  instalação e desinstalação fora do ambiente de desenvolvimento.

Até essas decisões, `multisend-settings-ui.ps1` permanece como fallback explícito. A opção mínima
recomendada para removê-lo é um utilitário Go pequeno e dedicado ao segredo, sem framework de UI;
as telas operacionais continuam no HTML servido pelo agente.

## Higiene de publicação

A varredura local da árvore não encontrou padrões de chave privada, OpenAI, GitHub, AWS ou bearer
token. Nenhum arquivo versionado atual excede 5 MiB, e o maior blob de todo o histórico medido tem
aproximadamente 85 KiB. `gitleaks` e `trufflehog` não estavam instalados; por isso o secret scan
formal do checklist continua pendente.

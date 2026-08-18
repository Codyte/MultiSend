# Histórico de envios LAN e pulls

## Por que existe um sidecar

O manifesto P2P descreve o arquivo, destino, identidade e chunks, mas não contém todo o contexto do
produto: subpasta remota, nome publicado, resultado de pasta, modo lab e diretório temporário de
cleanup. Retomar apenas pelo manifesto poderia publicar o arquivo no destino errado.

O agente grava um sidecar JSON local, versão 1, sem alterar o protocolo de rede nem o formato do
manifesto compartilhado.

Arquivos:

```text
%APPDATA%\MultiSend\history\jobs\job-<UnixNano>.json
%APPDATA%\MultiSend\history\pulls\pull-<UnixNano>.json
```

Os sidecars são gravados atomicamente na criação, ao anexar o manifesto e nas mudanças terminais de
estado. Eles contêm caminhos locais, endereço do peer e opções operacionais. Não contêm
`node_secret`, token de autenticação ou conteúdo do arquivo.

## Restauração de jobs P2P

Um job só volta como retomável quando:

- IDs de job e transferência seguem os formatos gerados pelo agente;
- arquivo-fonte e destino são absolutos/válidos;
- o manifesto está na pasta de manifests do remetente e seu nome corresponde à identidade;
- tipo, origem, destino, tamanho e plano de chunks conferem;
- o arquivo-fonte ainda existe e sua identidade recalculada (caminho, tamanho, modificação,
  destino, chunk e fingerprint de conteúdo) corresponde ao manifesto;
- opções de subpasta, publicação e resultado de pasta são válidas;
- quando há cleanup, ele contém o arquivo-fonte e corresponde a um staging gerenciado: diretório
  direto `multisend-pull-*` sob a pasta temporária do sistema ou `batch-*` sob
  `%TEMP%\MultiSend\staging`.

Jobs com todos os chunks concluídos voltam como `done`, mesmo se o ZIP temporário já foi removido.
Jobs incompletos coerentes voltam como `canceled`, prontos para retomada. Um sidecar válido cujo
manifesto ou fonte não pode ser verificado permanece visível como `failed`, mas não oferece
retomada. Sidecars com contexto estrutural inseguro são ignorados.

## Restauração de pulls

O pull persiste seu ID local, URL de origem, saída sob `receive_path`, peer e ID do job remoto. Se o
agente reiniciar durante a operação, o pull volta como `canceled`. Ao atualizar ou retomar, ele
consulta o endpoint do peer novamente; por isso a descoberta e o agente remoto precisam estar
disponíveis.

O agente remoto também restaura o job P2P pelo próprio sidecar. Isso mantém o mesmo ID usado por
`/remote-jobs/<id>` e permite que o pull local se reconecte sem inventar um novo envio.

## Retenção

Até 500 jobs terminais e 500 pulls terminais ficam na memória, além das operações ativas. Na
inicialização, são lidos os 500 sidecars mais recentes de cada tipo. A poda em memória não apaga os
arquivos no disco.

Registros terminais podem ser removidos individualmente pela interface ou com
`DELETE /jobs/<id>` e `DELETE /pulls/<id>`. A ação exige confirmação na UI e apaga somente o
sidecar local e a entrada em memória. Arquivos de origem, arquivos recebidos e manifests P2P não
são apagados. O staging gerenciado pertencente ao próprio job é removido nessa ação; caminhos
amplos, externos ou usados por outro fluxo são recusados. Operações ativas são recusadas.

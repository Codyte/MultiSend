# Diagnóstico e smokes locais

## Comandos

Com o agente instalado e em execução:

```powershell
& 'C:\Program Files\MultiSend\bin\multisend-agent.exe' --doctor
& 'C:\Program Files\MultiSend\bin\multisend-agent.exe' --lab-smoke
& 'C:\Program Files\MultiSend\bin\multisend-agent.exe' --download-smoke
```

O instalador expõe os mesmos fluxos com `-Doctor`, `-LabSmoke` e `-DownloadSmoke`, além do gate
`-Test`. Execute o `-Test` elevado quando for necessário conferir todos os atributos das regras de
firewall.

## `--doctor`

Verifica instalação, configuração, processo, API local, firewall, inicialização automática, menu do
Explorer e interfaces. Sem elevação, a presença das regras é consultada com `netsh`; os detalhes
completos continuam marcados como indisponíveis. `node_secret`, tokens e senhas são sempre
redigidos. `--print-config` também imprime somente uma configuração redigida.

## `--lab-smoke`

Cria uma execução exclusiva em `<receive_path>\_lab\receive\runs\run-<id>`, gera um arquivo determinístico
de 256 MiB, envia pela interface loopback, cancela após progresso real, retoma e valida:

- conclusão de todos os chunks sem falhas;
- manifesto de retomada;
- finalização da sessão receptora;
- existência e tamanho do arquivo recebido;
- remoção do job, manifesto e diretório exclusivo da execução.

Em caso de falha, a limpeza é tentada depois de cancelar o job. Se a API criar uma operação sem
retornar seu ID, o diretório é preservado para evitar apagar a fonte de um worker não identificado.

## `--download-smoke`

Sobe um servidor HTTP Range apenas em `127.0.0.1`, cria uma execução exclusiva em
`<receive_path>\_lab\download\runs\run-<id>`, baixa 256 MiB, cancela, retoma, confere o tamanho final
e remove o histórico e todo o diretório daquela execução.

## Limites de limpeza

A remoção recursiva só aceita um filho direto chamado `run-*` sob a raiz de runs criada pelo
smoke. O manifesto P2P só é removido quando está diretamente na pasta de manifests do remetente e
possui o sufixo esperado. Pastas comuns de recebimento e arquivos do usuário não entram nesse
escopo.

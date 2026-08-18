# CLIs de transferência dividida

`multisend.exe` e `multirecv.exe` são ferramentas diagnósticas para enviar um arquivo em duas
partes por endereços locais distintos. Eles não substituem o agente, a interface web ou os smokes
oficiais.

## Uso

No receptor:

```powershell
multirecv.exe -listen 0.0.0.0 -port 9000 -output C:\MultiSendLab
```

No emissor:

```powershell
multisend.exe `
  -file C:\Dados\arquivo.bin `
  -server 192.168.0.20:9000 `
  -cable-ip 192.168.0.10 `
  -wifi-ip 192.168.0.11 `
  -ratio 2:1
```

O receptor cria uma sessão `<arquivo>_<transfer_id>` dentro do diretório informado e preserva as
partes e o `manifest.json`. Esses CLIs não fundem o arquivo final, não implementam descoberta e não
oferecem retomada. Use o agente para operação normal.

## Contrato de integridade e autenticação

- as duas partes compartilham um `transfer_id` exclusivo por execução;
- índice, offset, tamanho individual e tamanho total fazem parte do header;
- cada parte inclui SHA-256 e só recebe ACK `done` após validação e persistência do manifesto;
- identificadores capazes de escapar do diretório de saída são rejeitados;
- a atualização do manifesto é serializada, mas a recepção dos payloads continua paralela;
- o emissor só conclui quando o ACK corresponde à transferência, ao chunk e ao tamanho enviados.

Autenticação é opcional para laboratório. Para habilitá-la, defina o mesmo segredo nos dois
processos sem colocá-lo na linha de comando:

```powershell
$env:MULTISEND_NODE_SECRET = '<segredo-compartilhado>'
```

Se autenticação for necessária em produção, use o agente com `require_auth=true`; o CLI não possui
configuração persistente nem administração de segredo.

## Efeitos e limites

Permitido:

- escutar no host/porta selecionados;
- ler somente o arquivo indicado no emissor;
- escrever sessões somente sob `-output`;
- substituir um chunk da mesma sessão durante uma repetição válida.

Não faz:

- exclusão automática de sessões;
- publicação ou fusão do arquivo final;
- abertura de firewall;
- descoberta de peers;
- chamadas a serviços externos.

## Validação

```powershell
go test ./cmd/multisend ./cmd/multirecv
```

Os testes usam loopback e `net.Pipe`, cobrindo ACK válido/inválido, duas partes concorrentes,
persistência do manifesto, traversal e divergência de hash.

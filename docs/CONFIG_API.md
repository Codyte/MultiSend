# API local de configuração

## Objetivo

`/api/v1/config` é o único contrato destinado à interface web para consultar e alterar a
configuração persistida do agente. Ele substitui a edição direta de `%APPDATA%\MultiSend\config.json`
pelas telas de produto.

A rota existe somente na API loopback e passa pelas mesmas validações de `Host` e `Origin` da UI
incorporada.

## GET `/api/v1/config`

Retorna três grupos:

- `config`: campos explicitamente editáveis;
- `runtime`: identidade, versão, portas selecionadas e faixas de porta somente para leitura;
- metadados de segurança e aplicação.

Exemplo reduzido:

```json
{
  "config": {
    "display_name": "PC-01",
    "receive_path": "C:\\Users\\Public\\Downloads\\MultiSend",
    "interface_policy": "auto",
    "require_auth": false,
    "remote_send_roots": []
  },
  "runtime": {
    "node_id": "PC-01-1234",
    "app_version": "0.1.0",
    "selected_ports": {
      "transfer": 56200,
      "discovery": 56211,
      "local_api": 56221,
      "control": 56231
    }
  },
  "secret_configured": true,
  "secret_exposed": false,
  "restart_required": false
}
```

`node_secret` nunca faz parte da resposta. `secret_configured` informa apenas se o agente possui um
segredo.

## PUT `/api/v1/config`

Recebe o objeto completo contido em `config`. Não é PATCH: campos booleanos omitidos seriam
ambíguos, então a UI deve primeiro executar GET, editar a cópia e enviar todos os campos.

Campos editáveis:

- identidade: `display_name`, `receive_path`;
- interfaces: política, atualização, flags e listas permitidas/manuais/ignoradas;
- limpeza e resultado de recebimento de pastas;
- pipeline, concorrência por interface, estratégia e percentual Ethernet;
- `require_auth` e `remote_send_roots`.

Após salvar, `restart_required` é `true`. Alguns componentes internos já recarregam subconjuntos do
arquivo, mas o contrato do produto considera a configuração completamente aplicada somente depois
de reiniciar o agente.

## Validações

- nome: 1 a 128 caracteres imprimíveis;
- pasta de recebimento: absoluta após normalização e nunca uma raiz de filesystem;
- a pasta de recebimento é criada antes da gravação;
- política: `auto`, `manual` ou `all`;
- atualização de interfaces: 2 a 60 segundos;
- listas: trim, remoção de vazios e duplicados sem diferenciar maiúsculas;
- resultado de pasta: `zip` ou `extract`;
- pipeline: `legacy` ou `pipelined`;
- concorrência por interface: 1 a 16;
- estratégia: `dynamic` ou `strict_split`;
- percentual Ethernet: 1 a 99;
- raízes remotas: devem existir, ser diretórios e não podem ser raízes de volume.

Uma requisição inválida retorna `400 invalid_config` e não modifica o arquivo. A gravação válida usa
arquivo temporário, flush e rename para evitar JSON parcialmente escrito.

## Autenticação

Ao mudar `require_auth` para `true`, o agente preserva o segredo existente. Se não houver segredo,
gera um localmente antes de salvar. O valor não é retornado para o navegador nem incluído em logs.

Exportação, revelação e pareamento de segredos continuam fora da UI web. Essas operações exigem um
fluxo nativo explícito antes de substituir o fallback PowerShell.

## Erros

Erros seguem o envelope comum da API:

```json
{
  "error": "invalid_config",
  "message": "receive_path: filesystem root is not allowed",
  "status": 400,
  "method": "PUT",
  "path": "/api/v1/config",
  "request_id": "req-..."
}
```

Outros códigos relevantes:

- `400 bad_json`;
- `400 receive_path_unavailable`;
- `403 invalid_host` ou `invalid_origin`;
- `405 method_not_allowed`;
- `500 config_read_failed` ou `config_save_failed`.

## Compatibilidade

`multisend-settings-ui.ps1` permanece no instalador temporariamente para exportação de segredo e
recuperação manual. Os atalhos principais já abrem `/ui/?view=settings`; a tela web não edita o JSON
diretamente.

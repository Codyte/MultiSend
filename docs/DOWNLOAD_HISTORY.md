# Histórico e recuperação de downloads

## Contrato

O agente reconstrói o histórico de downloads HTTP(S) durante a inicialização a partir dos arquivos
`manifest.json` já gravados pelo gerenciador. Nenhum banco de dados ou dependência adicional é
necessário.

Um manifesto só é aceito quando:

- o tipo é `internet_download` e o ID segue `dl-<UnixNano>`;
- a origem é uma URL HTTP(S) absoluta;
- o arquivo final e o manifesto permanecem dentro de `receive_path`;
- o manifesto está exatamente na pasta de sessão derivada do ID e do arquivo final;
- os chunks são sequenciais, contíguos, têm tamanhos válidos e cobrem o tamanho total;
- estados, tentativas e bytes concluídos são válidos.

Manifests inválidos ou de outros tipos são ignorados. Links simbólicos não são percorridos.

## Estados após reinício

- `done`: todos os chunks constam como concluídos e o arquivo final existe com o tamanho esperado;
- `canceled`: a operação foi interrompida e pode ser retomada;
- `failed`: algum chunk atingiu o limite automático de tentativas, mas uma retomada manual continua
  disponível.

Nenhum download restaurado volta como `running`: o agente não presume que workers sobreviveram ao
processo anterior.

Downloads novos nunca reutilizam silenciosamente um destino existente ou reservado por outro job.
Colisões recebem um sufixo no estilo `arquivo (2).ext`, considerando também pastas de sessão ainda
retomáveis. Os chunks são mesclados em um temporário no mesmo diretório; o arquivo final só é
publicado depois de fechamento, flush e validação de tamanho. Uma falha de merge preserva qualquer
arquivo final existente e não deixa uma saída parcial com o nome definitivo.

Se um chunk consta como concluído, mas o arquivo parcial correspondente não existe ou tem tamanho
incorreto, ele volta para `pending` e o manifesto reparado é salvo atomicamente. Quando o arquivo
final já está completo, os chunks parciais não são necessários para exibir o job concluído.

## Retenção e custo de inicialização

O agente mantém em memória até 500 jobs terminais, além de todos os jobs ativos. A remoção dessa
lista não apaga arquivos nem manifests. Após reiniciar, os 500 manifests mais recentes encontrados
são carregados novamente.

A busca para após 25.000 entradas do filesystem para limitar o custo de inicialização em uma pasta
de recebimento muito grande. O histórico completo continua no disco; itens fora dessa janela podem
não aparecer na interface.

## Limpeza

A limpeza só é permitida para jobs `done` cujo arquivo final e contadores conferem. Chunks e
manifesto precisam permanecer dentro da pasta de sessão esperada, sob `receive_path`.

Downloads direcionados a qualquer subpasta válida de `receive_path` podem ser limpos. A política
`keep_manifests` determina se o histórico continuará reidratável depois da remoção dos chunks.

Além da limpeza automática, `DELETE /downloads/<id>` remove explicitamente um download terminal
do histórico. A operação valida raiz, saída, diretório de sessão e caminho exato do manifesto;
apaga o manifesto e `chunk*.part`, mas preserva sempre o arquivo final. Um download ativo não pode
ser removido. Na interface, uma confirmação avisa que a capacidade de retomada será perdida.

## Outros tipos de operação

Envios LAN e pulls possuem dados e ciclos de vida diferentes e não reutilizam artificialmente este
manifesto. O sidecar local criado para esses fluxos está descrito em `OPERATION_HISTORY.md`.

# Imagens multimodais no Chat da TUI

> Status: proposta de ideia, não normativa.
>
> Direção escolhida: contrato multimodal comum no Hero, com tradução nativa em
> cada adapter e degradação explícita quando um harness ou modelo não oferecer a
> capacidade.
>
> Antes da implementação, esta proposta deve originar um novo ciclo com PRD,
> especificação de UI, ADR e especificações OpenSpec.

## 1. Contexto

O Chat da TUI do Hero transporta somente texto. O composer mantém uma `string`,
`conversation.Input` contém apenas `Text`, `harness.ExecuteRequest` contém apenas
`Prompt`, e a resposta normalizada usa `StreamDelta.Text` e
`ExecutionResult.Output`. Mesmo quando um protocolo nativo suporta imagens, o
contrato compartilhado atual não consegue preservar esse conteúdo como dado
tipado.

A proposta adiciona suporte bidirecional:

1. o usuário anexa ou cola uma imagem no composer e a envia junto do texto;
2. o adapter traduz a imagem para o protocolo nativo do harness;
3. imagens emitidas ou produzidas durante o turno retornam como assets tipados;
4. a TUI oferece preview e ações sobre o asset sem reduzir a imagem a texto ou
   base64 no transcript.

## 2. Objetivos

- Permitir anexar PNG, JPEG, GIF e WebP a uma mensagem do Chat.
- Preservar texto e imagens como partes ordenadas de um mesmo turno.
- Manter detalhes de protocolo dentro de cada adapter.
- Declarar capacidades por harness e, quando conhecido, por modelo.
- Receber imagens geradas como assets associados ao turno que as produziu.
- Permitir abrir, salvar, copiar o caminho e reutilizar uma imagem em outro
  prompt.
- Manter a TUI responsiva durante leitura, validação, conversão e preview.
- Funcionar em Linux e macOS, com fallback claro para clipboard, SSH e terminais
  sem protocolo gráfico.

## 3. Fora de escopo inicial

- Chamar diretamente APIs de geração de imagem fora dos harnesses.
- Criar uma segunda camada de autenticação, cobrança ou sessões por provider.
- Suportar áudio, vídeo, PDF ou anexos arbitrários no primeiro ciclo.
- Prometer geração de imagens para todo modelo que aceite imagens como entrada.
- Persistir automaticamente imagens no Git ou nos documentos de contexto.
- Transportar imagens pelo Telegram no primeiro incremento.
- Anexar imagens a comandos de controle `/hero-*` no primeiro incremento.

## 4. Princípios arquiteturais

### 4.1 Contrato comum, tradução local

O Hero deve conhecer conceitos neutros como `Attachment`, `Asset` e
`MediaCapability`. Codex, OpenCode, Claude e Cursor continuam responsáveis por
converter esses conceitos para seus protocolos nativos.

A TUI não deve construir payloads de provider, e os adapters não devem conhecer
Bubble Tea, clipboard, cards ou atalhos de teclado.

### 4.2 Referências imutáveis, não blobs no estado da TUI

Bytes e strings base64 não devem permanecer no `model` do Bubble Tea. A captura
é materializada uma única vez em armazenamento privado, e o restante do fluxo
usa uma referência imutável com metadados validados.

Isso evita copiar megabytes em atualizações do modelo, reconstruir base64 em
`View()` e aumentar o custo de scroll e redraw do transcript.

### 4.3 Capacidade explícita e falha fechada

Aceitar imagem como entrada e gerar imagem são capacidades independentes. Um
harness pode transportar imagens, mas o modelo escolhido pode ser text-only.

O Hero não deve remover silenciosamente um anexo, convertê-lo em uma descrição
inventada ou escolher outro modelo. Quando não houver suporte, o turno deve
falhar antes do envio com diagnóstico acionável. Qualquer uso da cadeia de
fallback existente precisa ser definido explicitamente no futuro PRD.

## 5. Modelo de domínio proposto

Os nomes finais pertencem ao ciclo de design, mas o contrato deve ter uma forma
equivalente a:

```go
type MediaKind string

const MediaKindImage MediaKind = "image"

type Attachment struct {
	ID       string
	Kind     MediaKind
	Name     string
	MIMEType string
	Path     string
	Size     int64
	Width    int
	Height   int
}

type Asset struct {
	Attachment
	Source    string // user, model, tool
	SessionID string
	TurnID    string
	Saved     bool
}
```

Extensões principais:

```go
type conversation.Input struct {
	Text        string
	Attachments []Attachment
	// campos existentes
}

type harness.ExecuteRequest struct {
	Prompt      string
	Attachments []Attachment
	// campos existentes
}

type harness.ExecutionResult struct {
	Output string
	Assets []Asset
	// campos existentes
}

type harness.StreamDelta struct {
	Kind  StreamKind
	Text  string
	Asset *Asset
	// campos existentes
}
```

Um novo `StreamKindAsset` transporta assets assim que forem conhecidos. O mesmo
asset também aparece em `ExecutionResult.Assets`, permitindo reparar perdas de
stream da mesma forma que os adapters hoje reparam texto parcial com o resultado
final.

O tipo compartilhado pode viver em `internal/harness` no primeiro ciclo. Se
clipboard, Telegram e outros transportes passarem a compartilhar as mesmas
operações, deve-se considerar um pacote pequeno e coeso `internal/media`, sem
dependências de TUI ou de adapters.

## 6. Fluxo de entrada

```text
clipboard / seletor / caminho colado
                 |
                 v
       captura assíncrona (tea.Cmd)
                 |
                 v
  validação + materialização privada
                 |
                 v
   chip no composer + Attachment ref
                 |
                 v
 conversation.Input.Attachments
                 |
                 v
 harness.ExecuteRequest.Attachments
                 |
                 v
       adapter -> protocolo nativo
```

### 6.1 Formas de anexar

- `Alt+V` ou `/attach-clipboard`: lê uma imagem do clipboard do sistema.
- `Alt+A` ou `/attach`: abre um seletor de arquivo.
- Drag-and-drop: quando o terminal cola um caminho, a TUI reconhece o evento de
  bracketed paste e oferece anexá-lo.
- Caminho explícito: `/attach ./design/mockup.png`, útil em SSH e multiplexers.

O atalho exato deve ser validado contra o keymap atual. Colar texto continua
funcionando normalmente. Um paste do terminal não carrega pixels de forma
portável; o comando de clipboard precisa consultar o clipboard gráfico do
sistema operacional.

### 6.2 Composer

Attachments aparecem como chips ou linhas compactas fora da `input string`:

```text
│ Compare este mockup com a implementação atual.
│ [image] checkout.png · PNG · 1440x900 · 820 KB  [x]
│ Build · gpt-5.4 · codex
```

O cursor textual não atravessa o conteúdo do chip. Backspace não remove um
attachment acidentalmente; a remoção usa foco/atalho explícito. O envio pode
conter somente imagem, desde que o PRD defina um prompt textual neutro ou que o
protocolo suporte uma mensagem sem texto.

## 7. Fluxo de saída

```text
evento/resultado nativo do harness
                 |
                 v
       adapter normaliza Asset
                 |
                 v
 StreamKindAsset + ExecutionResult.Assets
                 |
                 v
   asset associado à mensagem/turno
                 |
                 v
 card TUI -> preview / open / save / reuse
```

O comportamento básico não depende de pixels inline. Todo asset deve ter um
card navegável:

```text
│ Image generated
│ mockup-home.png · PNG · 1536x1024 · 1.8 MB
│ enter preview · o open · c copy path · a attach · s save
```

Ações propostas:

- abrir no visualizador padrão do sistema;
- copiar o caminho local;
- salvar/exportar para um destino escolhido;
- anexar ao próximo prompt;
- revelar no gerenciador de arquivos;
- registrar como artefato do ciclo após ação explícita.

Abrir aplicações externas deve suspender e restaurar corretamente a TUI. Leitura,
decodificação, geração de thumbnail e escrita em disco devem ser `tea.Cmd`; não
podem bloquear `Update()` ou ocorrer em `View()`.

## 8. Estratégias de visualização

### 8.1 Base obrigatória: card + visualizador externo

É a alternativa mais previsível em terminais, SSH e multiplexers. A imagem
continua utilizável mesmo quando nenhum protocolo gráfico estiver disponível.

### 8.2 Preview universal: mosaico Unicode

Um thumbnail com blocos Unicode e cores ANSI oferece compatibilidade ampla. Deve
ser calculado fora de `View()`, respeitar largura/altura do pane e degradar para
o card em terminais sem cores adequadas.

### 8.3 Preview avançado: Kitty, Sixel e iTerm2

Pixels reais podem ser adicionados posteriormente, com detecção best-effort e
configuração manual. Os protocolos são incompatíveis e interagem com alt-screen,
redraw, resize, scroll e limpeza de placements. O card permanece obrigatório,
mesmo quando o preview gráfico funciona.

## 9. Tradução por adapter

### 9.1 Codex

Primeiro adapter recomendado. O App Server atual expõe entradas `localImage` e
`image`, além de itens `imageView` e `imageGeneration`.

- Entrada: converter cada attachment local validado em `localImage` na lista
  ordenada de `turn/start.input`.
- Saída: normalizar `imageGeneration` e `imageView` em `Asset`, preservando
  `savedPath`, status, prompt revisado e erro seguro quando disponíveis.
- Compatibilidade: detectar o schema/capacidade da versão instalada; não assumir
  que toda versão do Codex oferece os mesmos itens.

### 9.2 OpenCode

Segundo adapter recomendado. A API aceita prompt composto por partes e anexos
referenciados por URI `file:` ou `data:`.

- Entrada: preferir `file:` absoluto para arquivos materializados no host do
  `opencode serve`; usar `data:` somente quando necessário.
- Saída: ampliar os tipos de `part` e o normalizador SSE para preservar partes
  de arquivo/imagem e resultados de tools como assets.
- Validar limites próprios do OpenCode e a capacidade do provider/model antes
  ou durante a admissão do prompt.

### 9.3 Claude

O adapter atual usa prompt posicional em modo headless. A UI interativa do
Claude aceita imagem no clipboard, mas essa funcionalidade não deve ser
automatizada.

- Caminho nativo: spike de protocolo usando `--input-format stream-json` e stdin
  estruturado, preservando supervisão, permissões, cancelamento e resume.
- Caminho temporário: referência a arquivo local e instrução explícita para o
  harness lê-lo, marcada como degradação e não como attachment nativo.
- A implementação nativa só entra após fixture reproduzível sem conta real.

### 9.4 Cursor

A CLI headless atualmente usada pelo Hero recebe um prompt posicional e não
oferece no contrato adotado um campo documentado de attachment.

- Caminho inicial: referência a arquivo acessível no workspace.
- Caminho futuro: suporte nativo apenas após capability probe e fixture estável.
- Se o modelo/harness não conseguir ler a imagem, falhar explicitamente; nunca
  remover o attachment silenciosamente.

## 10. Capabilities

O registro deve distinguir no mínimo:

```text
image_input_native
image_input_file_reference
image_output_native
image_output_file
supported_image_mime_types
max_attachment_bytes
```

Esses dados possuem duas fontes:

1. capacidade de transporte do adapter;
2. capacidade do modelo nativo, via descoberta ou catálogo quando conhecida.

Ausência de informação não equivale a suporte. A UI deve indicar antes do envio
quando um attachment será nativo, será entregue como referência de arquivo ou
não é suportado.

## 11. Armazenamento e ciclo de vida

### 11.1 Assets temporários

- Diretório de sessão com modo `0700` e arquivos `0600`.
- Localização fora de diretórios versionados por padrão.
- Identificador e nome gerados pelo Hero; o nome original é apenas metadado.
- Hash de conteúdo para deduplicação dentro da sessão.
- Manifesto pequeno para correlacionar asset, sessão, turno e origem.
- Política de expiração e limpeza em inicialização/encerramento.

### 11.2 Assets permanentes

Somente a ação `save` ou `register artifact` copia o arquivo para um destino
durável. Registrar um asset como artefato do ciclo deve reutilizar o serviço de
artefatos existente, sem tornar todo attachment um artefato automaticamente.

### 11.3 Validação e segurança

- Verificar magic bytes e decodificar a imagem; não confiar em extensão/MIME.
- Limitar quantidade, bytes, dimensões e pixels totais para evitar decompression
  bombs.
- Resolver symlinks e canonicalizar caminhos antes do envio.
- Não gravar base64, conteúdo da imagem ou caminhos sensíveis em logs.
- Não copiar imagens para `context-log.md`, `current-state.md` ou Git.
- Mostrar quando um caminho sai do workspace e exigir a política de permissão
  adequada.
- Sanitizar nomes e nunca reutilizar nomes fornecidos como caminho de escrita.

## 12. Persistência e sessões

Attachments fazem parte de um turno, não da identidade da sessão. Ao retomar uma
sessão nativa, o modelo pode conservar seu contexto, mas a TUI precisa de seu
próprio manifesto caso queira reconstruir cards após reinício.

O primeiro incremento pode manter cards apenas durante a execução da TUI. Se a
recuperação após reinício entrar no escopo, o manifesto deve ir para SQLite ou
para um arquivo privado transacional, com limpeza coordenada. Blobs não devem ser
armazenados no banco.

## 13. Relação com Telegram e outros transportes

Colocar attachments em `conversation.Input` mantém o núcleo neutro e permite
adicionar fotos do Telegram posteriormente. O IPC e o daemon permanecem text-only
no primeiro ciclo. Uma extensão futura deve transportar metadados e um arquivo
materializado privado, não base64 sem limite dentro de frames IPC.

## 14. Estratégia de testes

- Contratos de normalização e cópia defensiva de attachments/assets.
- Validação real de imagens pequenas em `t.TempDir()`.
- Rejeição de formato, MIME falso, symlink proibido, tamanho e dimensões.
- Fixtures de payload por adapter, sem chamadas a modelos reais.
- Codex: `localImage`, `imageView`, `imageGeneration`, falha e reparo final.
- OpenCode: file parts, SSE, resultado síncrono e recuperação de mensagens.
- Claude: spike/fixture stdin stream-json antes de alterar o adapter.
- Cursor: composição segura do fallback por caminho e erro de capability.
- TUI: captura assíncrona, chips, remoção, envio, cards, foco, resize e terminal
  pequeno.
- Golden/invariant tests para mosaico; evitar snapshots enormes de sequências
  Kitty/Sixel/iTerm2.
- Race tests para garantir que workers não mutem estado compartilhado da TUI.

## 15. Entrega incremental recomendada

### Fase 1 — Fundação e UX funcional

- contrato comum;
- serviço de assets temporários;
- capabilities;
- `/attach`, seletor, clipboard e caminho colado;
- chips no composer;
- cards, `open`, `copy path`, `save` e `attach again`;
- suporte inicial somente em Free Chat e follow-ups manuais.

### Fase 2 — Adapters nativos

- Codex input/output nativo;
- OpenCode input nativo e normalização de file/image parts;
- fallback explícito por arquivo para Claude e Cursor;
- erros por modelo sem visão.

### Fase 3 — Claude e maior paridade

- spike e implementação headless stream-json do Claude;
- descoberta de capabilities por modelo;
- persistência opcional dos cards;
- definição de comportamento da cadeia de fallback.

### Fase 4 — Preview e transportes adicionais

- mosaico Unicode;
- Kitty/Sixel/iTerm2 opcionais;
- imagens recebidas pelo Telegram;
- anexos em etapas/casos adicionais aprovados pelo produto.

## 16. Decisões que o futuro ciclo deve fechar

1. Diretório e retenção exatos dos assets temporários em projeto e Free Chat.
2. Limites comuns do Hero versus limites específicos de cada adapter.
3. Se um turno pode conter somente imagem, sem texto do usuário.
4. Ordem entre partes textuais e múltiplas imagens.
5. Comportamento do fallback quando o modelo primário não aceita imagem.
6. Escopo inicial: Free Chat somente ou também Research e agentes de estágio.
7. Persistência de cards após reinício da TUI.
8. Atalhos finais e comportamento em macOS, X11, Wayland, SSH e tmux.
9. Critério para classificar um arquivo criado por tool como saída do modelo.
10. Política para paths externos ao workspace.

## 17. Resultado esperado

Ao final, a TUI deixa de tratar toda conversa como duas strings e passa a tratar
cada turno como texto mais referências de mídia. A experiência permanece útil em
qualquer terminal por meio de cards e visualizador externo, enquanto adapters com
protocolos multimodais oferecem envio e retorno nativos sem contaminar o núcleo
do Hero com detalhes de provider.

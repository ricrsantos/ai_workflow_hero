# 10 - Gerenciamento de Artefatos

> **Versão do Documento:** 1.0
> **Status:** Rascunho
> **Aplica-se a:** Hero AI Loop Runtime
> **Idioma:** Português

---

# 1. Visão Geral

O Gerenciamento de Artefatos é responsável por gerenciar todo artefato produzido ao longo da execução de um ciclo de trabalho do Hero.

Os artefatos representam as saídas tangíveis geradas pelos agentes de IA durante o AI Development Loop.

Exemplos incluem:

* especificações;
* código-fonte;
* documentos de arquitetura;
* diagramas;
* relatórios de teste;
* screenshots;
* logs;
* manifestos de deployment;
* métricas.

Os artefatos são entidades de primeira classe dentro do Hero Runtime.

Eles não são simplesmente arquivos em disco.

Cada artefato possui identidade, metadados, ciclo de vida, propriedade (ownership) e histórico.

---

# 2. Visão

Um dos princípios centrais do Hero é que **toda saída importante deve se tornar um artefato gerenciado**.

Em vez de tratar os arquivos gerados como documentos isolados, o Hero mantém um catálogo completo de artefatos para cada ciclo de trabalho.

Isso possibilita:

* rastreabilidade;
* reprodutibilidade;
* auditoria;
* recuperação do ciclo de trabalho;
* comparação histórica.

---

# 3. Responsabilidades

O subsistema de Gerenciamento de Artefatos é responsável por:

* registrar artefatos;
* atribuir identificadores únicos;
* armazenar metadados;
* rastrear versões;
* rastrear propriedade (ownership);
* organizar artefatos por ciclo de trabalho;
* expor artefatos aos agentes;
* resolver referências de artefatos.

Ele **não** é responsável por:

* gerar artefatos;
* editar artefatos;
* executar ciclos de trabalho;
* persistir o estado do ciclo de trabalho.

---

# 4. Princípios de Design

## Artefato em Primeiro Lugar

Toda saída importante é um artefato gerenciado.

---

## Histórico Imutável

As versões históricas dos artefatos são preservadas.

---

## Rico em Metadados

Os artefatos carregam metadados descritivos.

---

## Delimitado ao Ciclo de Trabalho

Os artefatos pertencem a um ciclo de trabalho.

---

## Independente de Tecnologia

O armazenamento de artefatos é abstraído.

---

## Detectável

Os artefatos são pesquisáveis por metadados.

---

# 5. Ciclo de Vida do Artefato

Todo artefato segue um ciclo de vida comum.

```text id="k5ap0g"
Criado

    │

    ▼

Registrado

    │

    ▼

Disponível

    │

    ▼

Atualizado

    │

    ▼

Arquivado
```

O registro do artefato ocorre imediatamente após a criação.

---

# 6. Modelo de Artefato

Modelo sugerido em Go:

```go id="4k1lb6"
type Artifact struct {

    ID string

    WorkflowID string

    Stage string

    Name string

    Type ArtifactType

    Version int

    Path string

    CreatedAt time.Time

    UpdatedAt time.Time

    Metadata map[string]string

}
```

O Runtime é dono deste modelo.

---

# 7. Tipos de Artefato

Exemplos incluem:

Especificações.

Documentos de Arquitetura.

Documentos Markdown.

Código-Fonte.

Arquivos de Configuração.

Imagens.

Screenshots.

Relatórios de Teste.

Relatórios de Cobertura.

Logs de Execução.

Métricas.

Arquivos de Deployment.

A lista deve permanecer extensível.

---

# 8. Categorias de Artefato

Os artefatos podem ser agrupados por propósito.

```text id="c4jztm"
Pesquisa

Planejamento

Implementação

QA

UI QA

E2E

Deployment

Relatórios
```

As categorias simplificam a organização.

---

# 9. Propriedade do Artefato

Todo artefato pertence a exatamente um ciclo de trabalho.

Exemplo:

```text id="0zzh8g"
Ciclo de Trabalho

↓

Etapa de Planejamento

↓

open-spec.md
```

O compartilhamento entre ciclos de trabalho deve ser evitado.

---

# 10. Metadados do Artefato

Os metadados típicos incluem:

* identificador do ciclo de trabalho;
* etapa;
* agente produtor;
* timestamp de geração;
* versão;
* tipo de artefato;
* harness de origem;
* tags.

Os metadados possibilitam uma descoberta eficiente.

---

# 11. Registro de Artefatos

O Runtime mantém um registro.

Conceitualmente:

```text id="nzk6fa"
Ciclo de Trabalho

    │

    ├── discovery.md

    ├── architecture.md

    ├── open-spec.md

    ├── qa-report.md

    ├── ui-report.md

    └── e2e-report.md
```

O registro é independente do armazenamento físico.

---

# 12. Layout de Armazenamento

Evoluir / manter o layout de pastas atual do Hero

O Runtime não deve expor os detalhes de armazenamento externamente.

---

# 13. Registro de Artefatos

Sempre que um agente gera um artefato:

```text id="8p3j2o"
Agente

↓

Artefato Gerado

↓

Artifact Manager

↓

Registrar

↓

Publicar Evento artifact.generated
```

O registro é automático.

---

# 14. Versionamento de Artefatos

Os artefatos evoluem ao longo do tempo.

Exemplo:

```text id="xtmjlwm"
architecture.md

Versão 1

↓

Versão 2

↓

Versão 3
```

O histórico de versões deve permanecer acessível.

---

# 15. Estratégia de Versão

Abordagem recomendada:

* versões inteiras monotonicamente crescentes;
* versões históricas imutáveis;
* a versão mais recente marcada como atual.

Exemplo:

```text id="d0j4xw"
Versão 1

Arquivada

Versão 2

Arquivada

Versão 3

Atual
```

---

# 16. Referências de Artefato

Os componentes do ciclo de trabalho devem referenciar artefatos por identificador.

Exemplo:

```text id="3rq7lg"
artifact://workflow-001/open-spec
```

As referências são mais estáveis do que os caminhos de arquivo.

---

# 17. Resolução de Artefatos

Busca de artefato:

```text id="xhq7vn"
ID do Artefato

↓

Registro de Artefatos

↓

Metadados

↓

Localização Física
```

O Runtime resolve as referências de forma transparente.

---

# 18. Descoberta de Artefatos

Os artefatos devem ser pesquisáveis.

Exemplos:

Por etapa.

Por tipo.

Por agente.

Por data de criação.

Por tags.

Por ciclo de trabalho.

A busca é orientada por metadados.

---

# 19. Dependências de Artefato

Os artefatos podem depender de outros artefatos.

Exemplo:

```text id="jlwm6y"
PRD

↓

Open Spec

↓

Implementação

↓

Relatório de QA
```

As dependências possibilitam a rastreabilidade.

---

# 20. Grafo de Artefatos

Conceitualmente:

```text id="jxjlwm"
PRD

│

├── Arquitetura

│       │

│       └── Open Spec

│               │

│               └── Código-Fonte

│                       │

│                       └── Relatório de QA
```

Futuras versões do Runtime podem expor a visualização de dependências.

---

# 21. Relatórios Gerados

Toda etapa do ciclo de trabalho pode gerar relatórios.

Exemplos:

* research-summary.md;
* planning-summary.md;
* qa-report.md;
* ui-report.md;
* e2e-report.md.

Os relatórios são artefatos gerenciados.

---

# 22. Artefatos Binários

Os artefatos não se limitam a texto.

Exemplos:

* PNG;
* SVG;
* PDF;
* ZIP;
* MP4;
* screenshots;
* diagramas.

O modelo de metadados deve permanecer idêntico.

---

# 23. Disponibilidade do Artefato

Os artefatos ficam disponíveis imediatamente após o registro.

Os consumidores podem incluir:

* Workflow Engine;
* Camada de Conversação;
* futura Web UI;
* futura Desktop UI;
* futuras APIs.

O registro precede o consumo.

---

# 24. Integridade do Artefato

Os artefatos devem suportar a validação de integridade.

Possíveis mecanismos:

* hash SHA-256;
* tamanho do arquivo;
* timestamp de criação.

A verificação de integridade ajuda a detectar corrupção.

---

# 25. Eventos de Artefato

As operações de artefato publicam eventos.

Exemplos:

```text id="yjlwm8"
artifact.generated

artifact.updated

artifact.archived

artifact.deleted
```

Esses eventos se integram ao Sistema de Eventos do Runtime.

---

# 26. Política de Retenção

Os artefatos não devem ser excluídos automaticamente.

Futuras versões do Runtime podem implementar políticas de retenção configuráveis.

Comportamento padrão:

Preservar todos os artefatos.

---

# 27. Futuros Backends de Armazenamento

O armazenamento de artefatos deve ser abstraído.

Implementações possíveis:

* Sistema de arquivos local;
* BLOBs do SQLite;
* S3;
* Azure Blob Storage;
* Google Cloud Storage;
* Repositórios Git.

O Runtime permanece independente de armazenamento.

---

# 28. Interface do Repositório de Artefatos

Abstração sugerida:

```go id="r5m7yw"
type ArtifactRepository interface {

    Register(ctx context.Context, artifact *Artifact) error

    FindByID(ctx context.Context, id string) (*Artifact, error)

    FindByWorkflow(ctx context.Context, workflowID string) ([]Artifact, error)

    FindByStage(ctx context.Context, workflowID, stage string) ([]Artifact, error)

    Update(ctx context.Context, artifact *Artifact) error

}
```

As implementações podem variar sem afetar o Runtime.

---

# 29. Melhorias Futuras

Capacidades futuras potenciais incluem:

* diffing de artefatos;
* busca semântica;
* resumos de artefato gerados por IA;
* integração com Git;
* sincronização em nuvem;
* visualização de linhagem de artefatos;
* edição colaborativa.

A arquitetura deve acomodar essas capacidades.

---

# 30. Separação de Responsabilidades

O Workflow Engine:

* determina quando os artefatos são produzidos.

O Agent Orchestrator:

* solicita a geração de artefatos.

O subsistema de Gerenciamento de Artefatos:

* registra e gerencia os artefatos.

O subsistema de State Management:

* armazena o estado do ciclo de trabalho.

O Sistema de Eventos:

* distribui os eventos de artefato.

Cada subsistema tem uma única responsabilidade.

---

# 31. Princípios do Gerenciamento de Artefatos

O subsistema de Gerenciamento de Artefatos segue estes princípios.

## Saídas Gerenciadas

Toda saída importante se torna um artefato gerenciado.

---

## Histórico Imutável

As versões anteriores são preservadas.

---

## Orientado por Metadados

Os artefatos são identificados por metadados, em vez de caminhos de arquivo.

---

## Delimitado ao Ciclo de Trabalho

Os artefatos pertencem a um ciclo de trabalho específico.

---

## Independente de Armazenamento

O backend de armazenamento é um detalhe de implementação.

---

## Detectável

Os artefatos são pesquisáveis e referenciáveis.

---

## Preparado para o Futuro

O modelo suporta futuro armazenamento em nuvem, colaboração e gerenciamento de versões.

---

# 32. Declaração de Arquitetura

O subsistema de Gerenciamento de Artefatos é responsável pela identificação, registro, organização, versionamento e descoberta de todo artefato produzido durante o Hero AI Development Loop.

Ao tratar os artefatos como entidades de primeira classe, em vez de simples arquivos, o Hero Runtime oferece rastreabilidade completa, reprodutibilidade e visibilidade histórica em todo o ciclo de trabalho de desenvolvimento de software, permanecendo independente da tecnologia de armazenamento e dos harnesses de codificação com IA.
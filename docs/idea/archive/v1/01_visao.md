# 01 - Visão

> **Versão do Documento:** 1.0
> **Status:** Rascunho
> **Aplica-se a:** Hero AI Loop Runtime
> **Idioma:** Português

---

# 1. Visão

A ideia principal é evoluir o AI Workflow Hero para que ele possa passar a orquestrar diferentes Harness para o processo de desenvolvimento Agentico, porém mantendo a sua capacidade atual de operar dentro da interface do Cursor. Após esta evolução o Hero continuará capaz de operar como ele faz hoje, porém adicionamento ele vai adiquirir a capacidade de **Orquestrar** totalmente diferentes tipos de Harness, incluíndo o próprio Cursor CLI, sendo o Hero a interface conversacional com o usuário. 

O Hero vai avoluir para algo como um **AI Development Loop** que orquestra o Harness de escolha do usuário.

O Hero contínua não executando tarefas de desenvolvimento diretamente. Ele vai continuar sendo responsável por:

* orquestrar todo o fluxo de trabalho de desenvolvimento;
* coordenar agentes de IA especializados;
* gerenciar o contexto do projeto;
* rastrear o estado do fluxo de trabalho;
* solicitar aprovações;
* gerenciar custos de execução;

E vai adicionar as funcionalidades de:

* interagir com o usuário;
* integrar canais de comunicação externos;
* selecionar e controlar um ou mais harnesses de codificação com IA.

O Hero se torna o ponto único de coordenação do processo de desenvolvimento.

O usuário interage com o Hero.

O Hero interage com harnesses de codificação com IA.

Os harnesses interagem com modelos de IA.

---

# 2. Declaração do Problema

Os assistentes de codificação com IA atuais são excelentes na execução de tarefas isoladas, mas apresentam limitações importantes ao gerenciar todo um ciclo de vida de desenvolvimento de software.

Limitações típicas incluem:

* nenhuma orquestração persistente de fluxo de trabalho;
* nenhum ciclo de vida de desenvolvimento padronizado;
* nenhum estado de execução de longa duração;
* nenhuma coordenação multiagente;
* nenhum processo de aprovação centralizado;
* nenhuma propriedade de memória entre sessões;
* forte acoplamento entre a interação com o usuário e uma ferramenta de codificação específica.

Cada harness tem seu próprio fluxo de trabalho, o que dificulta a criação de uma experiência de desenvolvimento consistente.

O Hero resolve esse problema separando a **orquestração do fluxo de trabalho** da **execução de tarefas**.

---

# 3. Filosofia de Design

O Hero é construído em torno de um princípio fundamental:

> **O Hero é dono do fluxo de trabalho. Os harnesses executam o trabalho.**

Essa separação permite que o Hero se torne independente de qualquer plataforma de codificação com IA.

Em vez de incorporar a lógica de fluxo de trabalho dentro do Cursor, Claude Code, OpenCode ou qualquer futuro assistente de codificação, o Hero centraliza o processo de desenvolvimento em um runtime reutilizável.

Essa arquitetura permite:

* portabilidade;
* consistência;
* extensibilidade;
* suporte a múltiplos harnesses;
* integrações à prova de futuro.

---

# 4. Princípios Fundamentais

## 4.1 O Hero é Dono do Fluxo de Trabalho

O estado do fluxo de trabalho sempre pertence ao Hero.

O estágio atual, as aprovações pendentes, o histórico de execução, os artefatos gerados, os custos, os eventos e as decisões são mantidos pelo Hero.

O harness nunca se torna a fonte de verdade.

---

## 4.2 O Hero é Dono da Conversa

O usuário se comunica com o Hero.

O Hero decide:

* qual agente deve executar;
* qual harness deve ser usado;
* qual contexto deve ser enviado;
* como os resultados devem ser interpretados.

O usuário nunca deve precisar entender a orquestração interna dos agentes.

Os agentes se tornam detalhes de implementação.

---

## 4.3 Harnesses São Substituíveis

Todo harness deve implementar um contrato de execução comum.

Exemplos incluem:

* Cursor Agent CLI
* OpenCode
* Claude Code
* Futuros runtimes de codificação com IA

Substituir um harness por outro não deve exigir alterações no motor de fluxo de trabalho.

---

## 4.4 Agentes de IA São Trabalhadores Especializados

Os agentes têm uma única responsabilidade, conforme já ocorre na implementação atual do hero, porém agora será possível escolher inclusive o agente de Orchestração e o de Research, na versão atual estes agentes são escolhidos no chat do cursor. 

O Hero os coordena os agentes, eles nunca se coordenam diretamente entre si.

---

## 4.5 O Estado é Persistente

Os fluxos de trabalho de desenvolvimento podem executar por horas ou dias.

O Hero deve deve manter sua capacidade de:

* parar;
* retomar;
* recuperar;
* migrar;
* continuar a execução.

A execução do fluxo de trabalho nunca deve depender de uma sessão de chat ativa.

---

## 4.6 O Usuário Controla Decisões Críticas

Assim na implementação atual do Hero, o usuário deve continuar podendo adicionar aprovações humanas entre as etapas do processo. 

---

# 5. Visão de Futuro

O Hero não pretende se tornar mais um assistente de codificação.

Em vez disso, o Hero se torna uma plataforma de orquestração capaz de coordenar múltiplos sistemas de IA.

Conceitualmente:

```text
Usuário
    │
    ▼
Hero
    │
    ├── Fluxo de Trabalho
    ├── Contexto
    ├── Estado
    ├── Eventos
    ├── Aprovações
    ├── Custos
    └── Conversa
            │
            ▼
      Adaptador de Harness
            │
            ├── Cursor
            ├── OpenCode
            ├── Claude Code
            └── Futuros Harnesses
                    │
                    ▼
               Modelos de IA
```

O harness se torna intercambiável.

O Hero permanece constante.

---

# 6. Escopo

O Hero é responsável pelo ciclo de vida completo de desenvolvimento. Seguindo as mesmas etapas atualmente implementadas, quando habilitadas pelo usuário:

```text
Preparação do ciclo
      │
      ▼
Pesquisa / Descoberta
      │
      ▼
Planejamento
      │
      ▼
Implementação
      │
      ▼
QA
      │
      ▼
UI QA
      │
      ▼
E2E
      │
      ▼
Conclusão
```

Cada estágio é orquestrado pelo Hero.

---

# 7. Experiência do Usuário

O usuário interage com o Hero como um companheiro contínuo de desenvolvimento, semelhante a implementação atual.

Exemplo:

```text
Hero

Projeto carregado.

Estágio Atual:
Planejamento

O Planning Agent concluiu a especificação técnica.

Resumo

Duração:
18 minutos

Custo Estimado:
$0,42

Artefatos

✓ open-spec.md
✓ architecture.md

Continuar?

[Aprovar]
[Solicitar Alterações]
```

O usuário nunca precisa saber se o Planning Agent foi executado usando Cursor, OpenCode ou Claude Code.

Os detalhes de execução permanecem transparentes.

---

# 8. Visão de Longo Prazo

O Hero deve evoluir para se tornar uma plataforma completa de Desenvolvimento com IA.

Possíveis canais de interação incluem:

* Terminal User Interface (TUI)
* Command Line Interface (CLI)
* Painel Web
* Telegram
* Discord
* Aplicativos móveis
* Integrações com IDEs

Todas as interfaces se comunicam com o mesmo Hero Runtime.

O fluxo de trabalho permanece idêntico independentemente da interface utilizada.

---

# 9. Critérios de Sucesso

A arquitetura do Hero será considerada bem-sucedida quando:

* os fluxos de trabalho forem completamente independentes de qualquer harness específico;
* o mesmo fluxo de trabalho executar usando diferentes harnesses sem modificação;
* os usuários interagirem apenas com o Hero;
* o estado de desenvolvimento sobreviver a reinicializações do processo;
* as aprovações funcionarem em múltiplos canais de comunicação;
* os custos de execução forem totalmente rastreados;
* os artefatos forem gerenciados de forma centralizada;
* adicionar um novo harness exigir apenas a implementação de um novo adaptador.

---

# 10. Não-Objetivos

Os seguintes itens estão explicitamente fora das responsabilidades do Hero:

* implementar um modelo de linguagem;
* substituir harnesses de codificação com IA;
* substituir IDEs;
* substituir o Git;
* substituir plataformas de CI/CD;
* substituir frameworks de teste.

O Hero orquestra essas ferramentas em vez de substituí-las.

---

# 11. Declaração de Arquitetura

A arquitetura pode ser resumida em uma frase:

> **O Hero é um AI Development Loop Runtime multi-harness que é dono do fluxo de trabalho, da conversa, do estado e da experiência do usuário, enquanto delega a execução das tarefas a harnesses de codificação com IA intercambiáveis.**
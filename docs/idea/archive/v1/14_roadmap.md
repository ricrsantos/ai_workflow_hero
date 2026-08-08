# 14 - Roadmap

> **Versão do Documento:** 1.0
> **Status:** Rascunho
> **Aplica-se a:** Hero Runtime
> **Idioma:** Português

---# 14 - Roadmap

> **Versão do Documento:** 1.0
> **Status:** Rascunho
> **Aplica-se a:** Hero Runtime
> **Idioma:** Português

---

# 1. Visão Geral

Este documento define a evolução de longo prazo do Hero Runtime.

O objetivo do roadmap é fornecer uma visão estratégica para o projeto, ao mesmo tempo em que permite entregas incrementais por meio de marcos pequenos e bem definidos.

O roadmap é intencionalmente dividido em fases independentes.

Cada fase entrega um produto totalmente utilizável, ao mesmo tempo em que estabelece a base para capacidades futuras.

Espera-se que o Hero Runtime evolua de um orquestrador local de ciclos de trabalho de IA para uma Plataforma de Desenvolvimento com IA completa e independente de harness.

---

# 2. Visão

A visão de longo prazo do Hero é se tornar o sistema operacional para o desenvolvimento de software assistido por IA.

Em vez de estar preso a uma IDE, provedor de IA ou harness de codificação específico, o Hero é dono do ciclo de trabalho de desenvolvimento, enquanto as ferramentas de IA se tornam motores de execução intercambiáveis.

O roadmap reflete essa filosofia.

Cada marco aproxima o Hero de uma independência completa em relação às ferramentas de IA subjacentes.

---

# 3. Princípios de Desenvolvimento

O roadmap segue estes princípios.

## Runtime em Primeiro Lugar

O Runtime é sempre o centro da arquitetura.

---

## Independente de Harness

Suportar múltiplos harnesses de codificação com IA.

---

## Incremental

Entregar software utilizável em cada fase.

---

## Base Estável

As decisões arquiteturais centrais devem permanecer estáveis.

---

## Compatível com Versões Anteriores

Versões futuras devem preservar a compatibilidade dos ciclos de trabalho sempre que possível.

---

## Impulsionado pela Comunidade

A arquitetura deve incentivar extensões da comunidade.

---

# 4. Fase 0 – Fundação

Objetivo:

Estabelecer a arquitetura central.

Principais entregas:

* Hero CLI;
* instalação de projeto;
* configuração de projeto;
* definição do ciclo de trabalho;
* estrutura de diretórios;
* configuração local.

Esta fase estabelece a base técnica do projeto.

---

# 5. Fase 1 – Hero Runtime

Objetivo:

Criar o Runtime responsável por orquestrar os ciclos de trabalho.

Entregas:

* ciclo de vida do Runtime;
* Workflow Engine;
* State Management;
* Sistema de Eventos;
* Gerenciamento de Artefatos;
* Rastreamento de Custos;
* Camada de Conversação.

Ao final desta fase, o Hero é dono do ciclo de vida completo do ciclo de trabalho de IA.

---

# 6. Fase 2 – Terminal User Interface

Objetivo:

Substituir comandos de barra (slash commands) por uma interface Hero dedicada.

Entregas:

* TUI interativa;
* painel do ciclo de trabalho;
* visualização de progresso;
* aprovações;
* navegador de artefatos;
* resumos de execução;
* paleta de comandos.

A TUI se torna a interface de usuário primária.

---

# 7. Fase 3 – Cursor Adapter

Objetivo:

Integrar o Hero com o Cursor.

Entregas:

* Cursor Harness Adapter;
* gerenciamento de sessão;
* ciclo de vida de execução;
* coleta de uso;
* execução de conversação;
* integração com o ciclo de trabalho.

O Cursor se torna o primeiro harness de execução suportado.

---

# 8. Fase 4 – Suporte a Múltiplos Harnesses

Objetivo:

Suportar ambientes adicionais de codificação com IA.

Adaptadores potenciais:

* OpenCode;
* Claude Code;
* Gemini CLI;
* Codex CLI;
* futuros harnesses.

Nesta etapa, o Hero se torna independente de harness.

---

# 9. Fase 5 – Framework Multiagente

Objetivo:

Expandir o ecossistema interno de agentes.

Agentes iniciais:

* Discover Agent;
* Planning Agent;
* Backend Agent;
* Frontend Agent;
* QA Agent;
* UI QA Agent;
* E2E Agent;
* Judge Agent;
* Context Agent.

Futuros tipos de agente podem ser introduzidos sem modificar a arquitetura do Runtime.

---

# 10. Fase 6 – Integrações

Objetivo:

Possibilitar a comunicação com plataformas externas.

Integrações iniciais:

* Telegram;
* Discord;
* Git;
* GitHub;
* Slack.

As capacidades incluem:

* notificações;
* aprovações;
* monitoramento do ciclo de trabalho;
* comandos externos.

O Runtime permanece a autoridade central.

---

# 11. Fase 7 – Ecossistema de Artefatos

Objetivo:

Expandir o gerenciamento de artefatos.

Capacidades futuras:

* versionamento de artefatos;
* busca semântica;
* linhagem de artefatos;
* visualização de artefatos;
* grafo de dependências.

Os artefatos se tornam uma base de conhecimento navegável.

---

# 12. Fase 8 – Inteligência do Ciclo de Trabalho

Objetivo:

Aumentar a autonomia do ciclo de trabalho.

Capacidades potenciais:

* novas tentativas automáticas (retries);
* recomendações de execução;
* ciclos de trabalho adaptativos;
* otimização de ciclo de trabalho;
* seleção inteligente de agentes.

O Runtime se torna progressivamente mais autônomo.

---

# 13. Fase 9 – Aplicação Desktop

Objetivo:

Fornecer uma experiência desktop nativa.

Tecnologias potenciais:

* Go;
* Wails;
* integrações nativas com o sistema operacional.

Capacidades:

* gerenciamento gráfico do ciclo de trabalho;
* explorador de artefatos;
* painel do runtime;
* notificações.

A Desktop UI se comunica com o Runtime existente.

---

# 14. Fase 10 – Interface Web

Objetivo:

Expor o Hero por meio de um navegador.

Capacidades:

* monitoramento remoto do ciclo de trabalho;
* painéis;
* aprovações;
* visualização de artefatos;
* métricas.

A Web UI permanece como mais um cliente do Runtime.

---

# 15. Fase 11 – Colaboração em Equipe

Objetivo:

Suportar ciclos de trabalho colaborativos de IA.

Capacidades potenciais:

* ciclos de trabalho compartilhados;
* múltiplos revisores;
* aprovações em equipe;
* comentários;
* artefatos compartilhados;
* permissões baseadas em papéis (roles).

O Runtime evolui de execução para um único usuário para execução colaborativa.

---

# 16. Fase 12 – Runtime Distribuído

Objetivo:

Executar ciclos de trabalho em múltiplas máquinas.

Capacidades potenciais:

* agentes distribuídos;
* workers remotos;
* agendamento de carga de trabalho;
* execução em nuvem;
* replicação de ciclo de trabalho.

A arquitetura do Sistema de Eventos e do State Management já antecipa essa evolução.

---

# 17. Fase 13 – Plataforma em Nuvem

Objetivo:

Criar uma plataforma Hero hospedada.

Capacidades possíveis:

* Runtime gerenciado;
* armazenamento em nuvem;
* ciclos de trabalho hospedados;
* gerenciamento de equipe;
* monitoramento centralizado.

O Runtime local permanece totalmente suportado.

---

# 18. Fase 14 – Edição Enterprise

Capacidades enterprise potenciais:

* SSO;
* logs de auditoria;
* políticas empresariais;
* relatórios de conformidade;
* administração centralizada;
* implantações privadas.

A funcionalidade enterprise se constrói sobre a arquitetura existente.

---

# 19. Fase 15 – Marketplace de Plugins

Objetivo:

Permitir extensões da comunidade.

Possíveis categorias de plugin:

* integrações;
* templates de ciclo de trabalho;
* agentes;
* temas;
* painéis;
* alvos de deployment.

O Hero evolui para um ecossistema extensível.

---

# 20. Evolução do Provedor de IA

Embora o Hero execute principalmente por meio de harnesses, versões futuras podem suportar a execução direta opcional junto a provedores de IA.

Exemplos:

* OpenAI;
* Anthropic;
* Google;
* servidores de inferência local.

Essas capacidades devem complementar — e não substituir — a arquitetura do Harness Adapter.

---

# 21. Ecossistema MCP

Versões futuras devem suportar um ecossistema rico de servidores MCP.

Exemplos:

* Git;
* PostgreSQL;
* Kubernetes;
* AWS;
* automação de navegador;
* busca em documentação.

As integrações MCP expandem as capacidades do Hero sem aumentar a complexidade do Runtime.

---

# 22. Marketplace de Ciclos de Trabalho

Versões futuras podem fornecer templates de ciclo de trabalho reutilizáveis.

Exemplos:

* projeto de API REST;
* Microsserviços;
* aplicação móvel;
* aplicação de IA;
* pipeline de dados.

Os templates aceleram a inicialização de projetos.

---

# 23. Experiência do Usuário de Longo Prazo

A jornada do usuário de longo prazo se torna:

```text id="a4zr8q"
Iniciar o Hero

↓

Selecionar Ciclo de Trabalho

↓

Discutir Requisitos

↓

Revisar Progresso

↓

Aprovar Etapas

↓

Inspecionar Artefatos

↓

Receber Notificações

↓

Ciclo de Trabalho Concluído
```

Os usuários interagem apenas com o Hero durante todo o ciclo de vida de desenvolvimento.

---

# 24. Arquitetura de Longo Prazo

```text id="v8q4ds"
                  Hero

                   │

      ┌────────────┼────────────┐

      ▼            ▼            ▼

     TUI      Desktop UI     Web UI

                   │

                   ▼

          Camada de Conversação

                   ▼

             Hero Runtime

                   ▼

           Workflow Engine

                   ▼

         Harness Adapters

                   ▼

Cursor   OpenCode   Claude Code   Futuro
```

O Runtime permanece o centro arquitetural.

---

# 25. Métricas de Sucesso

O roadmap busca alcançar:

* independência completa de harness;
* ciclos de trabalho de IA reprodutíveis;
* execução transparente;
* arquitetura modular;
* ecossistema rico;
* escalabilidade enterprise.

Essas métricas orientam as decisões arquiteturais.

---

# 26. Fora do Escopo

O roadmap exclui intencionalmente:

* substituir harnesses de codificação com IA;
* implementar motores de inferência de LLM personalizados;
* se tornar uma IDE;
* substituir o Git.

O Hero orquestra o desenvolvimento em vez de substituir as ferramentas de desenvolvimento existentes.

---

# 27. Estratégia de Evolução

Toda fase do roadmap deve satisfazer três condições:

1. Entregar valor imediato ao usuário.

2. Preservar a consistência arquitetural.

3. Possibilitar expansão futura sem redesenho.

Essa estratégia minimiza a dívida técnica ao mesmo tempo em que maximiza a flexibilidade de longo prazo.

---

# 28. Filosofia Norteadora

O Hero evolui expandindo as capacidades de orquestração, em vez de aumentar o acoplamento.

À medida que o Runtime cresce, novas interfaces, harnesses, integrações e agentes devem se conectar por meio das abstrações arquiteturais existentes, em vez de introduzir implementações de casos especiais.

Essa filosofia garante a manutenibilidade de longo prazo.

---

# 29. Resumo do Roadmap

| Fase     | Objetivo Principal              |
| -------- | -------------------------------- |
| Fase 0   | Fundação                         |
| Fase 1   | Hero Runtime                     |
| Fase 2   | Terminal User Interface          |
| Fase 3   | Cursor Adapter                   |
| Fase 4   | Suporte a Múltiplos Harnesses    |
| Fase 5   | Framework Multiagente            |
| Fase 6   | Integrações                      |
| Fase 7   | Ecossistema de Artefatos         |
| Fase 8   | Inteligência do Ciclo de Trabalho|
| Fase 9   | Aplicação Desktop                |
| Fase 10  | Interface Web                    |
| Fase 11  | Colaboração em Equipe            |
| Fase 12  | Runtime Distribuído              |
| Fase 13  | Plataforma em Nuvem              |
| Fase 14  | Edição Enterprise                |
| Fase 15  | Marketplace de Plugins           |

---

# 30. Declaração de Arquitetura

O roadmap do Hero define o caminho evolutivo de um orquestrador local de ciclos de trabalho de IA para uma Plataforma de Desenvolvimento com IA abrangente e independente de harness.

Ao priorizar um Runtime estável, uma arquitetura modular, harnesses intercambiáveis, integrações extensíveis e múltiplas interfaces de usuário, o Hero estabelece uma base de longo prazo capaz de suportar desenvolvedores individuais, equipes distribuídas e desenvolvimento de software assistido por IA em escala enterprise, sem comprometer seus princípios arquiteturais centrais.

# 1. Visão Geral

Este documento define a evolução de longo prazo do Hero Runtime.

O objetivo do roadmap é fornecer uma visão estratégica para o projeto, ao mesmo tempo em que permite entregas incrementais por meio de marcos pequenos e bem definidos.

O roadmap é intencionalmente dividido em fases independentes.

Cada fase entrega um produto totalmente utilizável, ao mesmo tempo em que estabelece a base para capacidades futuras.

Espera-se que o Hero Runtime evolua de um orquestrador local de ciclos de trabalho de IA para uma Plataforma de Desenvolvimento com IA completa e independente de harness.

---

# 2. Visão

A visão de longo prazo do Hero é se tornar o sistema operacional para o desenvolvimento de software assistido por IA.

Em vez de estar preso a uma IDE, provedor de IA ou harness de codificação específico, o Hero é dono do ciclo de trabalho de desenvolvimento, enquanto as ferramentas de IA se tornam motores de execução intercambiáveis.

O roadmap reflete essa filosofia.

Cada marco aproxima o Hero de uma independência completa em relação às ferramentas de IA subjacentes.

---

# 3. Princípios de Desenvolvimento

O roadmap segue estes princípios.

## Runtime em Primeiro Lugar

O Runtime é sempre o centro da arquitetura.

---

## Independente de Harness

Suportar múltiplos harnesses de codificação com IA.

---

## Incremental

Entregar software utilizável em cada fase.

---

## Base Estável

As decisões arquiteturais centrais devem permanecer estáveis.

---

## Compatível com Versões Anteriores

Versões futuras devem preservar a compatibilidade dos ciclos de trabalho sempre que possível.

---

## Impulsionado pela Comunidade

A arquitetura deve incentivar extensões da comunidade.

---

# 4. Fase 0 – Fundação

Objetivo:

Estabelecer a arquitetura central.

Principais entregas:

* Hero CLI;
* instalação de projeto;
* configuração de projeto;
* definição do ciclo de trabalho;
* estrutura de diretórios;
* configuração local.

Esta fase estabelece a base técnica do projeto.

---

# 5. Fase 1 – Hero Runtime

Objetivo:

Criar o Runtime responsável por orquestrar os ciclos de trabalho.

Entregas:

* ciclo de vida do Runtime;
* Workflow Engine;
* State Management;
* Sistema de Eventos;
* Gerenciamento de Artefatos;
* Rastreamento de Custos;
* Camada de Conversação.

Ao final desta fase, o Hero é dono do ciclo de vida completo do ciclo de trabalho de IA.

---

# 6. Fase 2 – Terminal User Interface

Objetivo:

Substituir comandos de barra (slash commands) por uma interface Hero dedicada.

Entregas:

* TUI interativa;
* painel do ciclo de trabalho;
* visualização de progresso;
* aprovações;
* navegador de artefatos;
* resumos de execução;
* paleta de comandos.

A TUI se torna a interface de usuário primária.

---

# 7. Fase 3 – Cursor Adapter

Objetivo:

Integrar o Hero com o Cursor.

Entregas:

* Cursor Harness Adapter;
* gerenciamento de sessão;
* ciclo de vida de execução;
* coleta de uso;
* execução de conversação;
* integração com o ciclo de trabalho.

O Cursor se torna o primeiro harness de execução suportado.

---

# 8. Fase 4 – Suporte a Múltiplos Harnesses

Objetivo:

Suportar ambientes adicionais de codificação com IA.

Adaptadores potenciais:

* OpenCode;
* Claude Code;
* Gemini CLI;
* Codex CLI;
* futuros harnesses.

Nesta etapa, o Hero se torna independente de harness.

---

# 9. Fase 5 – Framework Multiagente

Objetivo:

Expandir o ecossistema interno de agentes.

Agentes iniciais:

* Discover Agent;
* Planning Agent;
* Backend Agent;
* Frontend Agent;
* QA Agent;
* UI QA Agent;
* E2E Agent;
* Judge Agent;
* Context Agent.

Futuros tipos de agente podem ser introduzidos sem modificar a arquitetura do Runtime.

---

# 10. Fase 6 – Integrações

Objetivo:

Possibilitar a comunicação com plataformas externas.

Integrações iniciais:

* Telegram;
* Discord;
* Git;
* GitHub;
* Slack.

As capacidades incluem:

* notificações;
* aprovações;
* monitoramento do ciclo de trabalho;
* comandos externos.

O Runtime permanece a autoridade central.

---

# 11. Fase 7 – Ecossistema de Artefatos

Objetivo:

Expandir o gerenciamento de artefatos.

Capacidades futuras:

* versionamento de artefatos;
* busca semântica;
* linhagem de artefatos;
* visualização de artefatos;
* grafo de dependências.

Os artefatos se tornam uma base de conhecimento navegável.

---

# 12. Fase 8 – Inteligência do Ciclo de Trabalho

Objetivo:

Aumentar a autonomia do ciclo de trabalho.

Capacidades potenciais:

* novas tentativas automáticas (retries);
* recomendações de execução;
* ciclos de trabalho adaptativos;
* otimização de ciclo de trabalho;
* seleção inteligente de agentes.

O Runtime se torna progressivamente mais autônomo.

---

# 13. Fase 9 – Aplicação Desktop

Objetivo:

Fornecer uma experiência desktop nativa.

Tecnologias potenciais:

* Go;
* Wails;
* integrações nativas com o sistema operacional.

Capacidades:

* gerenciamento gráfico do ciclo de trabalho;
* explorador de artefatos;
* painel do runtime;
* notificações.

A Desktop UI se comunica com o Runtime existente.

---

# 14. Fase 10 – Interface Web

Objetivo:

Expor o Hero por meio de um navegador.

Capacidades:

* monitoramento remoto do ciclo de trabalho;
* painéis;
* aprovações;
* visualização de artefatos;
* métricas.

A Web UI permanece como mais um cliente do Runtime.

---

# 15. Fase 11 – Colaboração em Equipe

Objetivo:

Suportar ciclos de trabalho colaborativos de IA.

Capacidades potenciais:

* ciclos de trabalho compartilhados;
* múltiplos revisores;
* aprovações em equipe;
* comentários;
* artefatos compartilhados;
* permissões baseadas em papéis (roles).

O Runtime evolui de execução para um único usuário para execução colaborativa.

---

# 16. Fase 12 – Runtime Distribuído

Objetivo:

Executar ciclos de trabalho em múltiplas máquinas.

Capacidades potenciais:

* agentes distribuídos;
* workers remotos;
* agendamento de carga de trabalho;
* execução em nuvem;
* replicação de ciclo de trabalho.

A arquitetura do Sistema de Eventos e do State Management já antecipa essa evolução.

---

# 17. Fase 13 – Plataforma em Nuvem

Objetivo:

Criar uma plataforma Hero hospedada.

Capacidades possíveis:

* Runtime gerenciado;
* armazenamento em nuvem;
* ciclos de trabalho hospedados;
* gerenciamento de equipe;
* monitoramento centralizado.

O Runtime local permanece totalmente suportado.

---

# 18. Fase 14 – Edição Enterprise

Capacidades enterprise potenciais:

* SSO;
* logs de auditoria;
* políticas empresariais;
* relatórios de conformidade;
* administração centralizada;
* implantações privadas.

A funcionalidade enterprise se constrói sobre a arquitetura existente.

---

# 19. Fase 15 – Marketplace de Plugins

Objetivo:

Permitir extensões da comunidade.

Possíveis categorias de plugin:

* integrações;
* templates de ciclo de trabalho;
* agentes;
* temas;
* painéis;
* alvos de deployment.

O Hero evolui para um ecossistema extensível.

---

# 20. Evolução do Provedor de IA

Embora o Hero execute principalmente por meio de harnesses, versões futuras podem suportar a execução direta opcional junto a provedores de IA.

Exemplos:

* OpenAI;
* Anthropic;
* Google;
* servidores de inferência local.

Essas capacidades devem complementar — e não substituir — a arquitetura do Harness Adapter.

---

# 21. Ecossistema MCP

Versões futuras devem suportar um ecossistema rico de servidores MCP.

Exemplos:

* Git;
* PostgreSQL;
* Kubernetes;
* AWS;
* automação de navegador;
* busca em documentação.

As integrações MCP expandem as capacidades do Hero sem aumentar a complexidade do Runtime.

---

# 22. Marketplace de Ciclos de Trabalho

Versões futuras podem fornecer templates de ciclo de trabalho reutilizáveis.

Exemplos:

* projeto de API REST;
* Microsserviços;
* aplicação móvel;
* aplicação de IA;
* pipeline de dados.

Os templates aceleram a inicialização de projetos.

---

# 23. Experiência do Usuário de Longo Prazo

A jornada do usuário de longo prazo se torna:

```text id="a4zr8q"
Iniciar o Hero

↓

Selecionar Ciclo de Trabalho

↓

Discutir Requisitos

↓

Revisar Progresso

↓

Aprovar Etapas

↓

Inspecionar Artefatos

↓

Receber Notificações

↓

Ciclo de Trabalho Concluído
```

Os usuários interagem apenas com o Hero durante todo o ciclo de vida de desenvolvimento.

---

# 24. Arquitetura de Longo Prazo

```text id="v8q4ds"
                  Hero

                   │

      ┌────────────┼────────────┐

      ▼            ▼            ▼

     TUI      Desktop UI     Web UI

                   │

                   ▼

          Camada de Conversação

                   ▼

             Hero Runtime

                   ▼

           Workflow Engine

                   ▼

         Harness Adapters

                   ▼

Cursor   OpenCode   Claude Code   Futuro
```

O Runtime permanece o centro arquitetural.

---

# 25. Métricas de Sucesso

O roadmap busca alcançar:

* independência completa de harness;
* ciclos de trabalho de IA reprodutíveis;
* execução transparente;
* arquitetura modular;
* ecossistema rico;
* escalabilidade enterprise.

Essas métricas orientam as decisões arquiteturais.

---

# 26. Fora do Escopo

O roadmap exclui intencionalmente:

* substituir harnesses de codificação com IA;
* implementar motores de inferência de LLM personalizados;
* se tornar uma IDE;
* substituir o Git.

O Hero orquestra o desenvolvimento em vez de substituir as ferramentas de desenvolvimento existentes.

---

# 27. Estratégia de Evolução

Toda fase do roadmap deve satisfazer três condições:

1. Entregar valor imediato ao usuário.

2. Preservar a consistência arquitetural.

3. Possibilitar expansão futura sem redesenho.

Essa estratégia minimiza a dívida técnica ao mesmo tempo em que maximiza a flexibilidade de longo prazo.

---

# 28. Filosofia Norteadora

O Hero evolui expandindo as capacidades de orquestração, em vez de aumentar o acoplamento.

À medida que o Runtime cresce, novas interfaces, harnesses, integrações e agentes devem se conectar por meio das abstrações arquiteturais existentes, em vez de introduzir implementações de casos especiais.

Essa filosofia garante a manutenibilidade de longo prazo.

---

# 29. Resumo do Roadmap

| Fase     | Objetivo Principal              |
| -------- | -------------------------------- |
| Fase 0   | Fundação                         |
| Fase 1   | Hero Runtime                     |
| Fase 2   | Terminal User Interface          |
| Fase 3   | Cursor Adapter                   |
| Fase 4   | Suporte a Múltiplos Harnesses    |
| Fase 5   | Framework Multiagente            |
| Fase 6   | Integrações                      |
| Fase 7   | Ecossistema de Artefatos         |
| Fase 8   | Inteligência do Ciclo de Trabalho|
| Fase 9   | Aplicação Desktop                |
| Fase 10  | Interface Web                    |
| Fase 11  | Colaboração em Equipe            |
| Fase 12  | Runtime Distribuído              |
| Fase 13  | Plataforma em Nuvem              |
| Fase 14  | Edição Enterprise                |
| Fase 15  | Marketplace de Plugins           |

---

# 30. Declaração de Arquitetura

O roadmap do Hero define o caminho evolutivo de um orquestrador local de ciclos de trabalho de IA para uma Plataforma de Desenvolvimento com IA abrangente e independente de harness.

Ao priorizar um Runtime estável, uma arquitetura modular, harnesses intercambiáveis, integrações extensíveis e múltiplas interfaces de usuário, o Hero estabelece uma base de longo prazo capaz de suportar desenvolvedores individuais, equipes distribuídas e desenvolvimento de software assistido por IA em escala enterprise, sem comprometer seus princípios arquiteturais centrais.
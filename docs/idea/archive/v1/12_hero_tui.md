# 12 - Hero Terminal User Interface (TUI)

> **Versão do Documento:** 1.0
> **Status:** Rascunho
> **Aplica-se a:** Hero Runtime
> **Idioma:** Português

---

# 1. Visão Geral

A Hero Terminal User Interface (TUI) é a interface de usuário primária do Hero Runtime.

Diferente dos assistentes de codificação com IA tradicionais, nos quais o usuário interage diretamente com o harness de codificação, o Hero se posiciona entre o usuário e o harness.

A TUI se torna o **ponto único de interação** para todo o AI Development Loop.

Os usuários se comunicam com o Hero.

O Hero se comunica com os harnesses de IA.

Essa separação é um dos princípios arquiteturais fundamentais do Hero Runtime.

---

# 2. Visão

O Hero **não** é um wrapper de linha de comando em torno do Cursor, OpenCode ou Claude Code.

O Hero é um Ambiente de Desenvolvimento com IA completo.

A interface de terminal deve fornecer uma experiência rica e interativa, semelhante às ferramentas modernas de desenvolvimento, permanecendo leve, orientada por teclado e multiplataforma.

O usuário nunca deve precisar abrir a interface do harness subjacente durante a execução normal do ciclo de trabalho.

---

# 3. Responsabilidades

A TUI é responsável por:

* apresentar informações do ciclo de trabalho;
* exibir o progresso da execução;
* coletar a entrada do usuário;
* apresentar perguntas;
* solicitar aprovações;
* exibir resumos;
* navegar pelos artefatos;
* mostrar os custos de execução;
* monitorar o status do ciclo de trabalho;
* exibir notificações do runtime.

Ela **não** é responsável por:

* executar ciclos de trabalho;
* tomar decisões sobre o ciclo de trabalho;
* comunicar-se com provedores de IA;
* persistir o estado do ciclo de trabalho.

---

# 4. Arquitetura de Alto Nível

```text id="kp8dzs"
                 Usuário
                  │
                  ▼
             Hero TUI
                  │
                  ▼
        Camada de Conversação
                  │
                  ▼
             Sistema de Eventos
                  │
                  ▼
           Hero Runtime
```

A TUI se comunica exclusivamente por meio da Camada de Conversação.

---

# 5. Princípios de Design

A Hero TUI segue estes princípios.

## Hero em Primeiro Lugar

Os usuários interagem com o Hero, não com o harness.

---

## Orientada por Teclado

Toda ação deve ser acessível pelo teclado.

---

## Orientada a Eventos

A interface reage a eventos do runtime.

---

## Não Bloqueante

Ciclos de trabalho de longa duração nunca travam a interface.

---

## Persistente

Os usuários podem se reconectar a um Runtime ativo.

---

## Multiplataforma

A mesma interface deve se comportar de forma consistente em Linux, macOS e Windows.

---

# 6. Layout Principal

A TUI é organizada em múltiplas regiões lógicas.

```text id="4bgr5v"
+-----------------------------------------------------------+
| Hero                                                     |
+-----------------------------------------------------------+
| Status do Ciclo de Trabalho                               |
+-----------------------------------------------------------+
| Conversação                                                |
|                                                           |
|                                                           |
|                                                           |
+-----------------------------------------------------------+
| Progresso da Etapa                                          |
+-----------------------------------------------------------+
| Resumo de Custo                                             |
+-----------------------------------------------------------+
| Linha de Comando                                             |
+-----------------------------------------------------------+
```

O layout exato pode evoluir preservando o modelo de interação.

---

# 7. Tela de Inicialização

Exemplo:

```text id="dly9uh"
Hero Runtime

Versão 1.0

Status do Runtime

Pronto

Nenhum ciclo de trabalho ativo.

Pressione Enter para começar.
```

A tela de inicialização fornece um ponto de entrada claro no Runtime.

---

# 8. Painel do Ciclo de Trabalho

Assim que um ciclo de trabalho é iniciado, a TUI exibe informações de alto nível.

Exemplo:

```text id="ijm7qq"
Projeto

AI Workflow Hero

Ciclo de Trabalho

Padrão

Etapa Atual

Planejamento

Status

Em Execução
```

Esse painel permanece visível durante toda a execução.

---

# 9. Área de Conversação

A Área de Conversação é o principal espaço de interação.

Exemplo:

```text id="yd8y3h"
Hero

Qual problema estamos tentando resolver?

Usuário

Quero criar uma plataforma de AI Workflow.

Hero

Quem são os usuários principais?
```

Os usuários conversam com o Hero, não com o harness subjacente.

---

# 10. Visualização de Progresso

O progresso do ciclo de trabalho deve permanecer visível.

Exemplo:

```text id="5rkk2k"
Preparação

✓

Pesquisa

✓

Planejamento

▶

Implementação

○

QA

○

UI QA

○

E2E

○
```

As atualizações de progresso ocorrem automaticamente.

---

# 11. Resumo da Etapa

Após cada etapa, o Hero apresenta um resumo.

Exemplo:

```text id="fkmjlwm"
Planejamento Concluído

Duração

18 minutos

Tokens de Entrada

24.000

Tokens de Saída

3.200

Custo Estimado

$0,42

Artefatos

✓ open-spec.md
```

O resumo precede qualquer solicitação de aprovação.

---

# 12. Diálogo de Aprovação

Certas etapas do ciclo de trabalho exigem aprovação.

Exemplo:

```text id="jlwmk2"
O planejamento foi concluído com sucesso.

Continuar?

[Aprovar]

[Solicitar Alterações]

[Pausar]

[Cancelar]
```

As ações disponíveis devem ser acessíveis por atalhos de teclado.

---

# 13. Área de Notificações

As notificações do runtime aparecem de forma discreta.

Exemplos:

```text id="0jlwmn"
Ciclo de trabalho iniciado.

QA concluído.

Ciclo de trabalho pausado.

Ciclo de trabalho retomado.

Artefato gerado.
```

As notificações nunca devem interromper a digitação.

---

# 14. Painel de Custos

As métricas de execução permanecem visíveis.

Exemplo:

```text id="jlwm7w"
Etapa Atual

$0,42

Total do Ciclo de Trabalho

$1,84

Tokens de Entrada

82.000

Tokens de Saída

9.600
```

Os valores são atualizados automaticamente após cada etapa concluída.

---

# 15. Painel de Artefatos

Os usuários podem inspecionar os artefatos do ciclo de trabalho.

Exemplo:

```text id="jlwm2q"
Artefatos

✓ discovery.md

✓ open-spec.md

✓ architecture.md

✓ qa-report.md
```

Selecionar um artefato deve abrir um visualizador.

---

# 16. Linha do Tempo do Ciclo de Trabalho

Versões futuras podem exibir o histórico de execução.

Exemplo:

```text id="jlwm4e"
09:00

Ciclo de Trabalho Iniciado

09:03

Preparação Concluída

09:28

Pesquisa Concluída

09:46

Planejamento Concluído
```

A linha do tempo melhora a observabilidade.

---

# 17. Status do Runtime

A TUI deve sempre indicar a saúde do Runtime.

Exemplo:

```text id="jlwm5r"
Runtime

Em Execução

Harness

Cursor

Ciclo de Trabalho

Executando

Eventos

Saudável
```

O status operacional aumenta a confiança do usuário.

---

# 18. Paleta de Comandos

A TUI deve expor uma interface de comandos pesquisável.

Exemplos:

```text id="jlwm6u"
Iniciar Ciclo de Trabalho

Pausar Ciclo de Trabalho

Retomar Ciclo de Trabalho

Cancelar Ciclo de Trabalho

Mostrar Artefatos

Mostrar Custos

Mostrar Histórico

Configurações
```

Os comandos devem ser fáceis de descobrir.

---

# 19. Atalhos de Teclado

Atalhos sugeridos:

```text id="jlwm8o"
Enter

Enviar

Ctrl+C

Cancelar Ciclo de Trabalho

Ctrl+P

Paleta de Comandos

Tab

Alternar Painel

Esc

Fechar Diálogo
```

Atalhos adicionais podem ser introduzidos ao longo do tempo.

---

# 20. Suporte a Múltiplos Ciclos de Trabalho

Versões futuras podem gerenciar múltiplos ciclos de trabalho.

Exemplo:

```text id="jlwm9n"
Ciclos de Trabalho

▶ Stock Hero

○ AI Workflow Hero

○ Indoor Location
```

A TUI deve permanecer responsiva independentemente da quantidade de ciclos de trabalho.

---

# 21. Reconexão

Se o Runtime reiniciar:

```text id="jlwm1v"
Hero Runtime

Recuperado

Ciclo de Trabalho Ativo Encontrado

Retomar?

[Sim]

[Não]
```

A recuperação deve parecer contínua.

---

# 22. Temas

A interface deve suportar múltiplos temas.

Temas iniciais:

* Escuro
* Claro

Temas futuros podem incluir:

* Alto Contraste
* Temas Personalizados

A personalização visual não deve afetar a funcionalidade.

---

# 23. Futuras Integrações

Embora a TUI seja a interface primária, a arquitetura suporta interfaces adicionais.

Exemplos:

* Desktop UI;
* Painel Web;
* Telegram;
* Discord;
* Slack;
* Assistentes de Voz.

Todas as interfaces se comunicam por meio da Camada de Conversação.

---

# 24. Tecnologia

A implementação inicial deve usar Go.

As bibliotecas recomendadas incluem:

* Bubble Tea
* Lip Gloss
* Bubbles

Essas bibliotecas fornecem um framework de TUI maduro e multiplataforma, permanecendo compatíveis com a arquitetura orientada a eventos do Hero.

As escolhas de bibliotecas podem evoluir sem afetar a arquitetura geral.

---

# 25. Separação de Responsabilidades

A TUI:

* apresenta informações;
* coleta a entrada do usuário.

A Camada de Conversação:

* gerencia as conversas.

O Workflow Engine:

* controla a execução do ciclo de trabalho.

O Runtime:

* coordena os subsistemas.

O Harness Adapter:

* comunica-se com os harnesses de IA.

Cada camada permanece independente.

---

# 26. Princípios da TUI

A Hero TUI segue estes princípios.

## O Hero é a Interface

O usuário interage exclusivamente com o Hero.

---

## Focada no Ciclo de Trabalho

A interface é otimizada para ciclos de trabalho de IA de longa duração.

---

## Orientada a Eventos

As atualizações se originam de eventos do Runtime.

---

## Responsiva

A interface permanece responsiva durante a execução.

---

## Observável

O progresso do ciclo de trabalho está sempre visível.

---

## Extensível

Futuros recursos devem se integrar naturalmente à interface.

---

# 28. Evolução Futura

A TUI é a primeira interface de usuário do Hero Runtime.

Sua arquitetura deve naturalmente suportar uma evolução futura para:

* Aplicações Desktop;
* Aplicações Web;
* Aplicações Móveis;
* Interfaces Colaborativas.

Essas futuras interfaces devem reutilizar a Camada de Conversação e o Runtime, sem alterar a execução do ciclo de trabalho.

---

# 29. Declaração de Arquitetura

A Hero Terminal User Interface é a superfície de interação primária do Hero Runtime.

Ela fornece uma experiência de usuário rica, orientada a eventos e centrada no ciclo de trabalho, que permite aos desenvolvedores gerenciar todo o AI Development Loop a partir de uma única interface, abstraindo a complexidade dos harnesses de codificação com IA subjacentes.

Ao tornar o Hero — e não o harness — o centro da experiência do usuário, a TUI estabelece a base para uma plataforma de desenvolvimento com IA unificada, extensível e independente de harness.
# 13 - Integrações

> **Versão do Documento:** 1.0
> **Status:** Rascunho
> **Aplica-se a:** Hero Runtime
> **Idioma:** Português

---

# 1. Visão Geral

O subsistema de Integrações permite que o Hero Runtime se comunique com serviços externos, ferramentas de desenvolvimento, plataformas de colaboração e sistemas de terceiros.

Seu propósito é estender as capacidades do Hero sem acoplar o Runtime a nenhuma tecnologia externa específica.

Todas as integrações são opcionais.

O Hero Runtime deve permanecer totalmente funcional mesmo quando nenhuma integração estiver configurada.

---

# 2. Visão

O Hero deve se tornar o orquestrador central do AI Development Loop.

Para alcançar isso, o Hero deve se integrar perfeitamente com as ferramentas que os desenvolvedores já utilizam.

Exemplos incluem:

* plataformas de comunicação;
* sistemas de controle de versão;
* rastreadores de issues;
* serviços de notificação;
* provedores de IA;
* plataformas de nuvem;
* ferramentas de monitoramento;
* sistemas de deployment.

Toda integração deve se comportar como um plugin em torno do Runtime, em vez de se tornar parte do seu núcleo.

---

# 3. Responsabilidades

O subsistema de Integrações é responsável por:

* comunicar-se com serviços externos;
* enviar notificações;
* receber eventos externos;
* expor informações do Runtime externamente;
* sincronizar o estado do ciclo de trabalho;
* traduzir protocolos externos em eventos do Hero;
* gerenciar o ciclo de vida das integrações.

Ele **não** é responsável por:

* execução do ciclo de trabalho;
* orquestração de IA;
* gerenciamento de estado;
* decisões de negócio.

---

# 4. Princípios de Design

## Opcional

Toda integração é opcional.

---

## Baseado em Plugins

As integrações são instaláveis de forma independente.

---

## Orientado a Eventos

As integrações se comunicam com o Runtime por meio de eventos.

---

## Seguro

As credenciais permanecem isoladas da lógica do Runtime.

---

## Extensível

Novas integrações não exigem modificações no Runtime.

---

## Independente de Tecnologia

O Runtime nunca depende de uma plataforma externa específica.

---

# 5. Arquitetura de Alto Nível

```text id="mwd2t9"
                 Hero Runtime

                       │

                       ▼

               Integration Manager

                       │

      ┌────────────────┼────────────────┐

      ▼                ▼                ▼

 Telegram Plugin   Git Plugin     GitHub Plugin

      ▼                ▼                ▼

 APIs Externas    Git CLI/API     GitHub API
```

O Runtime se comunica apenas com o Integration Manager.

---

# 6. Ciclo de Vida da Integração

Cada integração segue o mesmo ciclo de vida.

```text id="vsyk2d"
Instalada

    │

    ▼

Configurada

    │

    ▼

Habilitada

    │

    ▼

Em Execução

    │

    ▼

Desabilitada

    │

    ▼

Removida
```

O Runtime gerencia o ciclo de vida de forma independente da execução do ciclo de trabalho.

---

# 7. Integration Manager

O Integration Manager é responsável por:

* carregar as integrações;
* validar a configuração;
* iniciar as integrações;
* interromper as integrações;
* rotear os eventos de integração;
* expor as capacidades de integração.

Ele atua como o ponto central de coordenação de todas as integrações externas.

---

# 8. Interface de Integração

Interface sugerida em Go:

```go id="t1v7ps"
type Integration interface {

    Name() string

    Version() string

    Initialize(ctx context.Context) error

    Start(ctx context.Context) error

    Stop(ctx context.Context) error

    Capabilities() Capabilities

}
```

Toda integração implementa a mesma interface.

---

# 9. Integração via Eventos

As integrações se comunicam exclusivamente por meio do Sistema de Eventos do Runtime.

Exemplo:

```text id="i7z1mu"
Ciclo de Trabalho Concluído

      │

      ▼

workflow.completed

      │

      ▼

Integration Manager

      │

      ▼

Integração com o Telegram

      │

      ▼

Enviar Notificação
```

O Workflow Engine nunca se comunica diretamente com as integrações.

---

# 10. Integrações de Notificação

As plataformas de notificação podem incluir:

* Telegram;
* Discord;
* Slack;
* Microsoft Teams;
* E-mail;
* SMS.

Essas integrações se inscrevem nos eventos do Runtime e entregam notificações externamente.

---

# 11. Integração com o Telegram

Espera-se que o Telegram se torne uma das principais integrações do Hero.

As possíveis capacidades incluem:

* notificações do ciclo de trabalho;
* solicitações de aprovação;
* status do ciclo de trabalho;
* resumos de artefatos;
* alertas do runtime;
* comandos do usuário.

Exemplo:

```text id="m7q1hv"
Ciclo de Trabalho Concluído

↓

Notificação no Telegram

↓

Usuário

↓

Aprovar

↓

approval.received

↓

Workflow Engine
```

A integração com o Telegram nunca interage diretamente com o Workflow Engine.

---

# 12. Integração com o Discord

O Discord fornece capacidades semelhantes.

Exemplos:

* atualizações do ciclo de trabalho;
* resumos de etapa;
* solicitações de aprovação;
* métricas de execução;
* notificações do runtime.

Versões futuras podem suportar componentes de mensagem interativos.

---

# 13. Integrações de Controle de Versão

As possíveis integrações incluem:

* Git;
* GitHub;
* GitLab;
* Azure DevOps.

As capacidades podem incluir:

* criação de branches;
* geração de commits;
* criação de pull requests;
* solicitações de code review.

Essas ações são disparadas por meio de eventos do Runtime.

---

# 14. Integrações de Rastreamento de Issues

Futuras integrações podem incluir:

* Jira;
* Linear;
* GitHub Issues;
* Azure Boards.

Possíveis capacidades:

* criar issues;
* atualizar o status de issues;
* anexar artefatos;
* sincronizar o progresso do ciclo de trabalho.

---

# 15. Integrações de Nuvem

Exemplos:

* AWS;
* Azure;
* Google Cloud.

Capacidades futuras:

* deployment;
* upload de artefatos;
* backups do ciclo de trabalho;
* gerenciamento de segredos.

As integrações de nuvem permanecem opcionais.

---

# 16. Integrações MCP

O Hero deve suportar servidores externos do Model Context Protocol (MCP).

Exemplos:

* Git MCP;
* PostgreSQL MCP;
* Filesystem MCP;
* Browser MCP;
* Slack MCP;
* Servidores MCP personalizados.

O Runtime interage com os servidores MCP por meio de integrações dedicadas, em vez de incorporar a lógica do MCP nos componentes do ciclo de trabalho.

---

# 17. Futuras Integrações com Provedores de IA

Embora a execução de IA seja delegada aos harnesses, futuras capacidades do Runtime podem se integrar diretamente com:

* OpenAI;
* Anthropic;
* Google;
* servidores de inferência local.

Essas integrações devem permanecer separadas dos Harness Adapters.

---

# 18. Comandos Recebidos

Algumas integrações podem receber comandos.

Exemplo:

```text id="cw1qaf"
Telegram

↓

Pausar Ciclo de Trabalho

↓

Integração

↓

workflow.pause.requested

↓

Barramento de Eventos

↓

Workflow Engine
```

Os comandos recebidos se tornam eventos do Runtime.

---

# 19. Notificações Enviadas

O Runtime publica eventos.

As integrações decidem como apresentá-los.

Exemplos:

```text id="g7nyq4"
workflow.started

workflow.completed

approval.required

artifact.generated

runtime.failed
```

A apresentação permanece específica de cada integração.

---

# 20. Segurança

As integrações frequentemente exigem credenciais.

Exemplos:

* chaves de API;
* tokens OAuth;
* tokens de bot;
* segredos de webhook.

Requisitos de segurança:

* nunca registrar credenciais em log;
* criptografar configurações sensíveis quando apropriado;
* isolar segredos dos artefatos do ciclo de trabalho;
* evitar expor credenciais aos agentes de IA.

O gerenciamento de credenciais pertence ao Integration Manager.

---

# 21. Configuração

Exemplo de configuração:

```yaml id="iw4q3v"
integrations:

  telegram:

    enabled: true

  discord:

    enabled: false

  github:

    enabled: true
```

Cada integração é dona de sua própria configuração.

---

# 22. Instalação de Plugins

A Hero CLI deve suportar a instalação de integrações.

Exemplo:

```bash id="9b4srl"
hero install --tools telegram
```

Ou:

```bash id="ndb8iw"
hero install --tools github
```

A instalação deve registrar a integração junto ao Runtime.

---

# 23. Descoberta de Capacidades

As integrações expõem suas capacidades.

Exemplo:

```go id="2y7jks"
type Capabilities struct {

    Notifications bool

    Commands bool

    FileUpload bool

    Authentication bool

}
```

O Runtime adapta seu comportamento com base nas capacidades disponíveis.

---

# 24. Tratamento de Falhas

As falhas de integração nunca devem interromper a execução do ciclo de trabalho.

Exemplo:

```text id="0jytcq"
Telegram Offline

↓

Notificação Falhou

↓

Tentar Novamente

↓

Registrar Evento em Log

↓

Ciclo de Trabalho Continua
```

O Runtime permanece resiliente a falhas externas.

---

# 25. Monitoramento

O Integration Manager deve expor métricas operacionais.

Exemplos:

* integrações ativas;
* requisições falhas;
* contagem de novas tentativas (retries);
* latência de mensagens;
* falhas de autenticação.

Essas métricas melhoram a observabilidade do Runtime.

---

# 26. Futuro Marketplace

Futuras versões do Hero podem suportar um marketplace de integrações.

Exemplo:

```text id="9k5o2n"
Hero Marketplace

↓

Instalar Integração

↓

Habilitar

↓

Configurar

↓

Usar
```

As integrações desenvolvidas pela comunidade devem seguir a mesma interface.

---

# 27. Separação de Responsabilidades

O Workflow Engine:

* controla a execução do ciclo de trabalho.

O Sistema de Eventos:

* distribui os eventos.

O Integration Manager:

* coordena as integrações.

Cada Integração:

* comunica-se com uma plataforma externa.

Os Serviços Externos:

* permanecem fora do limite do Runtime.

As responsabilidades permanecem isoladas.

---

# 28. Princípios de Integração

O subsistema de Integrações segue estes princípios.

## Baseado em Plugins

Toda integração é instalável de forma independente.

---

## Orientado a Eventos

A comunicação ocorre por meio de eventos do Runtime.

---

## Opcional

O Runtime funciona sem integrações externas.

---

## Seguro

As credenciais permanecem isoladas dos ciclos de trabalho de IA.

---

## Substituível

As integrações podem ser adicionadas ou removidas sem afetar o Runtime.

---

## Extensível

Novas plataformas se integram por meio de um contrato comum.

---

# 29. Evolução Futura

Futuras versões do Runtime podem suportar:

* plataformas de colaboração bidirecionais;
* provedores de identidade corporativos;
* pipelines de deployment;
* sistemas de monitoramento;
* integrações com CRM;
* plataformas de documentação;
* conectores empresariais personalizados.

Essas capacidades devem ser implementadas como integrações, e não como modificações no Runtime.

---

# 30. Declaração de Arquitetura

O subsistema de Integrações estende o Hero Runtime além do terminal, fornecendo um mecanismo seguro, baseado em plugins e orientado a eventos para se comunicar com plataformas externas.

Ao isolar serviços de terceiros por trás de interfaces de integração padronizadas, o Hero possibilita notificações, colaboração, automação, conectividade em nuvem e extensibilidade empresarial, preservando os princípios centrais do Runtime de modularidade, portabilidade e independência de harness.
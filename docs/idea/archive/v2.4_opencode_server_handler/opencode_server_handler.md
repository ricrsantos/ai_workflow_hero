# OpenCode Server Lifecycle Management

## Objetivo

Garantir que todo processo `opencode serve` iniciado pelo Hero seja rastreável e encerrado corretamente, evitando servidores órfãos após encerramentos inesperados do Hero.

A responsabilidade deve ficar no **OpenCode Adapter**, pois ele é quem cria e gerencia o processo `opencode serve`.

## Funcionamento

1. **Inicialização**

   * O OpenCode Adapter inicia o `opencode serve`.
   * Obtém o `PID` e a porta efetivamente utilizada.
   * Registra o servidor no `hero.db`.

2. **Registro**

   O registro deve conter, no mínimo:

   ```text
   opencode_server
   ├── id
   ├── pid
   ├── port
   ├── project_path
   └── started_at
   ```

3. **Shutdown normal**

   * O Hero solicita o encerramento do Harness.
   * O OpenCode Adapter envia `SIGTERM` ao processo registrado.
   * Aguarda o encerramento.
   * Se necessário, utiliza `SIGKILL` como fallback.
   * Remove o registro correspondente do `hero.db`.

4. **Shutdown inesperado**

   * Se o Hero for encerrado por `SIGINT`, `SIGTERM` ou `SIGHUP`, o lifecycle manager deve tentar encerrar os servidores conhecidos.
   * Caso o processo do Hero morra abruptamente e não consiga executar o cleanup, o registro permanece no banco.

5. **Startup / Garbage Collection**

   * Ao iniciar, o Hero consulta os registros de servidores OpenCode criados anteriormente.
   * Para cada registro:

     * verifica se o `PID` ainda existe;
     * confirma que o processo corresponde ao OpenCode registrado;
     * encerra o processo, se ainda estiver ativo;
     * remove o registro obsoleto.
   * Somente depois dessa limpeza o Hero inicia normalmente.

6. **Watchdog opcional**

   * Um watchdog periódico pode verificar servidores registrados.
   * Caso encontre um servidor órfão ou inconsistente, encerra o processo e remove seu registro.

## Schema

```text
                         ┌──────────────────────┐
                         │         Hero         │
                         └──────────┬───────────┘
                                    │
                              Start OpenCode
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │  OpenCode Adapter    │
                         └──────────┬───────────┘
                                    │
                                    │ spawn
                                    ▼
                         ┌──────────────────────┐
                         │   opencode serve    │
                         │                      │
                         │ PID: 28160           │
                         │ PORT: 42417          │
                         └──────────┬───────────┘
                                    │
                                    │ register
                                    ▼
                         ┌──────────────────────┐
                         │       hero.db        │
                         │                      │
                         │ pid                  │
                         │ port                 │
                         │ project_path         │
                         │ started_at           │
                         └──────────────────────┘


        NORMAL SHUTDOWN
        ─────────────────

Hero shutdown
      │
      ▼
OpenCode Adapter
      │
      ├── SIGTERM
      │
      ├── wait
      │
      ├── SIGKILL (fallback)
      │
      └── remove hero.db record


        UNEXPECTED SHUTDOWN
        ────────────────────

Hero crashes
      │
      ▼
OpenCode process remains
      │
      ▼
Next Hero startup
      │
      ▼
Read hero.db
      │
      ▼
Check PID + process identity
      │
      ├── process exists → terminate
      │
      └── process absent → remove stale record
      │
      ▼
Start Hero normally
```

## Localização

A implementação deve ficar no **OpenCode Adapter**, preferencialmente separada em um componente interno de lifecycle/process management:

```text
internal/
└── harness/
    └── opencode/
        ├── adapter.go
        ├── server.go          # lifecycle do opencode serve
        ├── stream.go          # consumo dos eventos
        └── ...
```

O `server.go` deve ser responsável por:

* iniciar o `opencode serve`;
* descobrir PID/porta;
* registrar no `hero.db`;
* monitorar o processo;
* executar shutdown;
* remover o registro;
* realizar cleanup de servidores órfãos no startup.

## Princípio importante

O Hero **não deve procurar e matar processos `opencode` arbitrariamente**.

Ele deve encerrar **somente os processos que ele próprio iniciou e registrou**.

Isso evita interferir em uma eventual instância do OpenCode iniciada manualmente pelo usuário ou por outro processo.

## Evolução futura

Se o OpenCode passar a oferecer um mecanismo confiável de `idle timeout` ou auto-shutdown para servidores criados via `serve`, ele poderá complementar esse mecanismo. Entretanto, o lifecycle do Hero deve continuar existindo como mecanismo de segurança.

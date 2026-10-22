# Logoff de usuários

Aplicativo gráfico para Windows que ajuda a identificar e encerrar sessões de usuário abertas no computador local. Ele mostra o nome da conta, o número da sessão e o estado — **Ativa** ou **Desconectada** — e oferece a ação de logoff após uma confirmação.

![Ícone do Logoff de usuários](logout-icon-clean.png)

O programa é voltado a operadores que precisam administrar sessões sem usar comandos no Prompt de Comando. A interface concentra a tarefa em uma lista, uma pesquisa e o botão **Encerrar sessão selecionada**.

## Funcionalidades

- Lista as sessões de usuário encontradas no computador local, incluindo sessões desconectadas.
- Pesquisa por qualquer trecho do nome da conta, sem diferenciar maiúsculas de minúsculas.
- Permite selecionar uma ou várias sessões.
- Mostra quantas sessões estão selecionadas e pede confirmação antes de iniciar o logoff.
- Atualiza a lista depois da operação e informa se alguma sessão não pôde ser encerrada.
- Carrega as sessões em segundo plano e permite navegar por listas grandes usando a roda do mouse ou a barra de rolagem.
- Inclui o ícone e os recursos visuais dentro do executável.

O programa executa somente o logoff das sessões selecionadas. Ele não reinicia nem desliga o computador. O logoff fecha os aplicativos da sessão e alterações que não foram salvas podem ser perdidas.

## Como usar

1. Execute `logoff-ususarios.exe`.
2. Localize a sessão na lista ou digite parte do nome no campo de pesquisa.
3. Selecione a sessão ou sessões desejadas.
4. Confira a seleção e clique em **Encerrar sessão selecionada**.
5. Confirme a operação na janela apresentada.

### Seleção e atalhos

| Ação | Como fazer |
| --- | --- |
| Selecionar uma sessão | Clique nela. |
| Adicionar ou remover uma sessão da seleção | `Ctrl` + clique. |
| Selecionar um intervalo | Clique na primeira sessão e depois `Shift` + clique na última. |
| Selecionar os resultados visíveis da pesquisa | `Ctrl+A`. |
| Ir para a pesquisa | `Ctrl+F`. O texto atual fica selecionado para substituição. |
| Ampliar a seleção para a sessão acima ou abaixo | Com a lista em foco, use `Shift` + `↑` ou `Shift` + `↓`. |
| Desfazer a seleção | `Esc`. |
| Percorrer uma lista longa | Use a roda do mouse ou a barra de rolagem à direita. |

Ao alterar a pesquisa, as sessões já selecionadas continuam selecionadas, mesmo enquanto estiverem ocultas pelo filtro. Elas também permanecem incluídas no logoff; confira o total selecionado antes de confirmar. `Ctrl+A` acrescenta os resultados visíveis à seleção existente. Sessões que já não existem são removidas da seleção quando a lista é atualizada.

## Requisitos e permissões

- O executável de distribuição é para **Windows 10 de 64 bits (x64)**.
- Não é necessário instalar Go, DLLs ou arquivos adicionais para executar o arquivo da pasta `build`.
- Para encerrar sessões de outros usuários, execute o aplicativo com uma conta que tenha as permissões necessárias no computador, normalmente uma conta de administrador. A elevação não garante a operação se as políticas ou permissões do sistema a bloquearem.

## Arquivos para distribuição

A pasta `build` contém o executável e o arquivo `LEIA-ME.txt`. O executável é independente e já contém o ícone e o manifesto visual.

O pacote `build.zip` contém o conteúdo da pasta `build`. As Releases do GitHub disponibilizam dois arquivos: `logoff-ususarios.exe` para uso direto e `logoff-ususarios.zip` com os arquivos da pasta `build`.

## Compilação

É necessário Go 1.22 ou superior. O arquivo `rsrc_windows_amd64.syso` incorpora o manifesto e o ícone ao executável e já está incluído no projeto. Para compilar o executável Windows x64:

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -ldflags="-H windowsgui" -o logoff-ususarios.exe .
```

## Testes

Execute os testes unitários e de recursos do ícone com:

```sh
go test ./...
```

Para executar com o detector de corridas e medir a cobertura:

```sh
go test -race -cover ./...
```

## CI e publicação de Releases

O workflow em `.github/workflows/ci-release.yml` executa os testes e compila o aplicativo para Windows x64 em cada push e pull request. Um push na **branch padrão** publica uma Release se o assunto do commit for `Versão` seguido por grupos numéricos separados por pontos, por exemplo:

```text
Versão 1.0
Versão 1.0.0
```

A versão é usada como nome e tag da Release. Para publicar, o workflow precisa da permissão `contents: write`, já declarada no job de Release.

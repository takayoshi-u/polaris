# Polaris Model Context Protocol (MCP) サーバー

[![TypeScript](https://img.shields.io/badge/TypeScript-5.7-blue.svg)](https://www.typescriptlang.org/)
[![Model Context Protocol](https://img.shields.io/badge/MCP-1.1.0-purple.svg)](https://modelcontextprotocol.io/)
[![Node.js](https://img.shields.io/badge/Node.js-22+-green.svg)](https://nodejs.org/)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue.svg)](https://www.docker.com/)

**Polaris MCP サーバー** は、プロジェクト管理 Web アプリケーション「 **Polaris** 」と LLM / AI アシスタント（Claude Desktop、Cursor、Antigravity など）を接続するための [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) 公式準拠サーバーです。

AI エージェントは本サーバー経由で Polaris のプロジェクト、課題、タスク、アジャイル・スプリント、バーンダウンチャート、Wiki ドキュメントへ直接アクセスし、情報の取得・分析・更新を自律的に行うことができます。

---

## 主な機能

- **プロジェクト分析 & 要約**: プロジェクト一覧の取得に加え、総ストーリーポイント（SP）、タスク完了率（%）、予定／実績工数（h）、アクティブスプリントの自動ロールアップ集計（`get_project_summary`）を提供。
- **課題（親）の追跡・管理**: 課題の一覧取得・検索、詳細閲覧、作成、更新（プレモーテム事前リスク評価やポストモーテム振り返りを含む）。
- **アジャイルタスク（子）の操作**: タスクの一覧・詳細閲覧、新規作成、ステータスや進捗率（0〜100%）、ストーリーポイント、実績工数の更新。
- **スプリント & スクラム支援**: スプリント一覧、リアルタイム・バーンダウンチャート取得、および直前スプリントの終了日や設定期間に基づいた日付自動計算付きスプリント作成。
- **Wiki ナレッジベース**: プロジェクト Wiki ページのツリー一覧取得、内容閲覧、新規作成および更新（単一ツール `save_wiki_page` で自動分岐）。
- **柔軟なデュアル認証**: 個人用アクセストークン（PAT）またはメールアドレス／パスワードによるセッション自動ログインに対応。
- **軽量・セキュアな Docker サポート**: `node:22-alpine` ベースのマルチステージビルドを採用し、非 root ユーザー（`node`）で最小構成実行。

---

## 環境変数一覧

| 環境変数名 | 必須 | デフォルト値 | 説明 |
| :--- | :---: | :---: | :--- |
| `POLARIS_BASE_URL` | 任意 | `http://localhost:8080/api/v1` | Polaris バックエンド REST API のベース URL |
| `POLARIS_API_TOKEN` | 条件付* | *(未設定)* | 個人用アクセストークン（PAT）。推奨認証方式 |
| `POLARIS_EMAIL` | 条件付* | *(未設定)* | アカウントのメールアドレス（自動ログイン用） |
| `POLARIS_PASSWORD` | 条件付* | *(未設定)* | アカウントのパスワード（自動ログイン用） |

> **認証についての注意**:
> `POLARIS_API_TOKEN` または `POLARIS_EMAIL` ＆ `POLARIS_PASSWORD` のいずれか一方が設定されている必要があります。

---

## Google アカウント連携ユーザーの利用手順

Google OAuth（ソーシャルログイン）で Polaris にサインアップ・ログインしているユーザーは、初期状態ではパスワードが設定されていません。そのため、以下のいずれかの方法で認証を行ってください。

### 推奨: 個人用アクセストークン (PAT) を発行して利用する

1. ブラウザで Polaris にログインします。
2. 画面左下または右上のユーザーメニューから **「アカウント設定」** を開きます。
3. **「個人用アクセストークン (PAT)」** タブを選択し、**「新規トークン発行」** をクリックします。
4. トークン名（例: `Claude Desktop MCP`）と必要な権限スコープ（`read:projects`, `write:issues`, `write:tasks` など、または全権限）を選択して発行します。
5. 表示されたトークン（`polaris_pat_...`）をコピーし、環境変数 `POLARIS_API_TOKEN` に設定します。

```bash
export POLARIS_API_TOKEN="polaris_pat_your_token_value_here"
```

### 代替案: アカウント設定画面でパスワードを登録する

1. Polaris の **「アカウント設定」** ➔ **「セキュリティ / パスワード設定」** を開きます。
2. アカウントに任意のログインパスワードを登録します。
3. 設定完了後、環境変数 `POLARIS_EMAIL`（Google アカウントのメールアドレス）と `POLARIS_PASSWORD`（登録したパスワード）を指定して MCP サーバーを起動できます。

---

## クイックスタート

### 1. Docker で起動する場合（推奨）

コンテナイメージをビルドします：

```bash
cd mcp
docker build -t polaris-mcp:latest .
```

対話型 stdio でコンテナを実行します：

```bash
docker run -i --rm \
  -e POLARIS_BASE_URL="http://host.docker.internal:8080/api/v1" \
  -e POLARIS_API_TOKEN="polaris_pat_your_token_value_here" \
  polaris-mcp:latest
```

> **ヒント (Linux 環境)**:
> Linux ホスト上で実行する場合、コンテナからホストマシンの `localhost:8080` にアクセスするために `--add-host=host.docker.internal:host-gateway` を付与してください。

---

### 2. Node.js で直接実行する場合

前提条件: Node.js 22 以上および npm がインストールされていること。

```bash
cd mcp
npm install
npm run build
```

環境変数を指定して起動します：

```bash
export POLARIS_BASE_URL="http://localhost:8080/api/v1"
export POLARIS_API_TOKEN="polaris_pat_your_token_value_here"

# ビルド成果物を起動
npm start
# または
node dist/index.js
```

---

## クライアント設定例

### 1. Claude Desktop

OS ごとの設定ファイルに MCP サーバー定義を追加します：
- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/Claude/claude_desktop_config.json`

#### Docker を使用する場合の設定例

```json
{
  "mcpServers": {
    "polaris": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "-e", "POLARIS_BASE_URL=http://host.docker.internal:8080/api/v1",
        "-e", "POLARIS_API_TOKEN=polaris_pat_your_token_value_here",
        "polaris-mcp:latest"
      ]
    }
  }
}
```

#### Node.js を直接使用する場合の設定例

```json
{
  "mcpServers": {
    "polaris": {
      "command": "node",
      "args": ["/絶対パス/polaris/mcp/dist/index.js"],
      "env": {
        "POLARIS_BASE_URL": "http://localhost:8080/api/v1",
        "POLARIS_API_TOKEN": "polaris_pat_your_token_value_here"
      }
    }
  }
}
```

---

### 2. Cursor

プロジェクトルートの `.cursor/mcp.json` に設定を記述します：

```json
{
  "mcpServers": {
    "polaris": {
      "command": "node",
      "args": ["/絶対パス/polaris/mcp/dist/index.js"],
      "env": {
        "POLARIS_BASE_URL": "http://localhost:8080/api/v1",
        "POLARIS_API_TOKEN": "polaris_pat_your_token_value_here"
      }
    }
  }
}
```

---

### 3. Antigravity

Antigravity の MCP 設定ファイル（または MCP サーバー管理画面）で以下のように登録します：

```json
{
  "polaris": {
    "command": "node",
    "args": ["/絶対パス/polaris/mcp/dist/index.js"],
    "env": {
      "POLARIS_BASE_URL": "http://localhost:8080/api/v1",
      "POLARIS_API_TOKEN": "polaris_pat_your_token_value_here"
    }
  }
}
```

---

## 提供ツール（全 18 ツール）

### 1. プロジェクト (`Projects`)

| ツール名 | パラメータ | 説明 |
| :--- | :--- | :--- |
| `list_projects` | *(なし)* | アクセス可能な Polaris プロジェクト一覧を取得します。 |
| `get_project_summary` | `projectId` (UUID) | 課題数・タスク数・完了率（%）・総ストーリーポイント・予実工数・アクティブスプリントなどのプロジェクト全体要約を取得します。 |
| `archive_project` | `projectId` (UUID) | プロジェクトをアーカイブします（読み取り専用に保護され、デフォルトのアクティブ一覧から非表示化されます）。 |
| `unarchive_project` | `projectId` (UUID) | アーカイブされたプロジェクトを復元（アーカイブ解除）します。 |

### 2. 課題 (`Issues`)

| ツール名 | パラメータ | 説明 |
| :--- | :--- | :--- |
| `list_issues` | `projectId` (UUID), `status_id?`, `type_id?`, `search?`, `sprint_id?` | プロジェクト内の課題一覧を取得します（検索キーワード・ステータス・種別・スプリントによる絞り込み対応）。 |
| `get_issue` | `projectId` (UUID), `issueId` (UUID) | 課題の詳細情報、配下タスク一覧、カスタム項目値、事前リスク（premortem）、事後振り返り（postmortem）を取得します。 |
| `create_issue` | `projectId` (UUID), `title`, `description?`, `type_id?`, `template_id?`, `premortem?`, `postmortem?`, `label_ids?`, `create_initial_task?` | 新規課題を作成します。`create_initial_task: true` を指定すると同名初期タスクを同時生成します。 |
| `update_issue` | `projectId` (UUID), `issueId` (UUID), `title?`, `description?`, `state?`, `type_id?`, `premortem?`, `postmortem?`, `label_ids?` | 既存課題のタイトル、説明、状態（open/closed）、種別、振り返り内容などを更新します。 |

### 3. タスク (`Tasks`)

| ツール名 | パラメータ | 説明 |
| :--- | :--- | :--- |
| `list_tasks` | `projectId` (UUID), `issue_id?`, `status_id?`, `sprint_id?` | プロジェクト内の全タスク一覧を取得します（親課題・ステータス・スプリントによる絞り込み対応）。 |
| `get_task` | `projectId` (UUID), `taskId` (UUID) | タスクの詳細情報（予定／実績開始終了日、工数、ストーリーポイント、担当者など）を取得します。 |
| `create_task` | `projectId` (UUID), `issue_id` (UUID), `title`, `description?`, `status_id?`, `progress?`, `story_points?`, `planned_hours?`, `actual_hours?`, `sprint_id?`, `type_id?`, `planned_start_date?`, `planned_end_date?`, `assignees?` | 親課題の配下に新規タスクを作成します。 |
| `update_task` | `projectId` (UUID), `taskId` (UUID), `title?`, `description?`, `status_id?`, `progress?`, `story_points?`, `planned_hours?`, `actual_hours?`, `sprint_id?`, `type_id?`, `planned_start_date?`, `planned_end_date?`, `assignees?` | タスクの進捗率（0〜100%）、実績工数、ストーリーポイント、ステータス、担当者などを更新します。 |

### 4. スプリント & アジャイル (`Sprints`)

| ツール名 | パラメータ | 説明 |
| :--- | :--- | :--- |
| `list_sprints` | `projectId` (UUID) | プロジェクト内の全スプリント一覧を取得します。 |
| `get_sprint_burndown` | `sprintId` (UUID) | スプリントのバーンダウンチャート実績データ（日次残余 SP、理想線、実績線）を取得します。 |
| `create_sprint` | `projectId` (UUID), `name?`, `start_date?`, `end_date?`, `duration_days?`, `goal?` | スプリントを作成します。開始日・終了日・スプリント名を省略した場合は、直前スプリントの終了日や設定期間に基づき自動算出します。 |

### 5. Wiki ドキュメント (`Wiki`)

| ツール名 | パラメータ | 説明 |
| :--- | :--- | :--- |
| `list_wiki_pages` | `projectId` (UUID) | プロジェクトの Wiki ドキュメント一覧（階層構造）を取得します。 |
| `get_wiki_page` | `wikiId` (UUID) | 指定した Wiki ページの Markdown 本文およびメタデータを取得します。 |
| `save_wiki_page` | `projectId` (UUID), `wiki_id?`, `title`, `content`, `parent_id?` | Wiki ページを作成・更新します。`wiki_id` を省略した場合は新規作成、指定した場合は既存ページの更新を行います。 |

---

## トラブルシューティング

### 1. Docker コンテナから Polaris API に接続できない場合
- **macOS / Windows**: `POLARIS_BASE_URL` に `http://host.docker.internal:8080/api/v1` を指定してください。
- **Linux**: Docker コマンドに `--add-host=host.docker.internal:host-gateway` オプションを追加し、`http://host.docker.internal:8080/api/v1` を指定してください。
- Docker Compose ネットワーク内に参加させる場合は、サービス名（例: `http://api:8080/api/v1`）を指定できます。

### 2. `Polaris API Error (403): permission denied` が発生する場合
- 使用している個人用アクセストークン（PAT）に、要求された操作に対応するスコープ（例: 課題作成なら `write:issues`、プロジェクト取得なら `read:projects`）が付与されているか確認してください。
- 該当ユーザーが対象プロジェクトのメンバーとして登録されているか、および適切なロール（Editor または Admin）を持っているか確認してください。

### 3. `Authentication required` エラーが発生する場合
- `POLARIS_API_TOKEN` または `POLARIS_EMAIL` ＆ `POLARIS_PASSWORD` が正しく環境変数に渡されているか確認してください。
- Claude Desktop や Cursor の設定ファイル（JSON）内で構文エラーがないか、`env` セクションが正しく記述されているか確認してください。

---

## 開発とテスト

単体テストの実行：
```bash
cd mcp
npm test
```

TypeScript ビルド：
```bash
cd mcp
npm run build
```

---

## ライセンス

本 MCP サーバーは [MIT License](../LICENSE) の下で提供されています。

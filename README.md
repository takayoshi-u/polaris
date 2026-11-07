# Polaris — プロジェクト管理 Web アプリケーション

**Polaris** は、「プロジェクト ＞ 課題（親） ＞ タスク（子）」の明確な階層構造、フィールド単位のきめ細かな権限・テンプレートエンジン、課題スイムレーン型マトリクス・カンバンボード、予実2段バー対応のガントチャートを備えたプロジェクト管理 Web アプリケーションです。

## ドキュメント一覧

システムの詳細仕様・データベース設計・API仕様・外部連携手順は以下のドキュメントを参照してください。

- **[クイックスタートガイド (`QUICKSTART.md`)](QUICKSTART.md)** — 最短1分で動かせる Docker 起動手順・チュートリアル
- **[確定仕様書 (`docs/specification.md`)](docs/specification.md)** — システム全体のドメインルール、権限モデル、画面・機能仕様
- **[ER図・テーブル定義書 (`docs/er.md`)](docs/er.md)** — PostgreSQL 17 データベースの Mermaid ER図および全 29 テーブルの定義（全主キー UUID v7）
- **[OpenAPI 3.0.3 仕様書 (`docs/openapi.yaml`)](docs/openapi.yaml)** — バックエンド REST API（`/api/v1`）の全エンドポイント・リクエスト／レスポンススキーマ定義
- **[テスト品質保証ガイドライン (`docs/testing_lessons_and_guidelines.md`)](docs/testing_lessons_and_guidelines.md)** — 発生した不具合の事後分析（Post-Mortem）および再発防止テスト設計原則
- **[スクラム実践スタートガイド (`docs/scrum_starter_guide.md`)](docs/scrum_starter_guide.md)** — スプリント・ストーリーポイント・バーンダウン・DoDを活用した実践ガイド
- **[エージェント開発規約・PR運用規程 (`AGENTS.md`)](AGENTS.md)** — Git ブランチ戦略、Pull Request 運用ルール、エージェントチーム役割分担
- **[Model Context Protocol サーバー (`mcp/README.md`)](mcp/README.md)** — Claude Desktop、Cursor、Antigravity 等の AI アシスタントと連携する MCP サーバーの導入・利用手順

---

## 技術スタック

### バックエンド (`backend/`)
- **言語**: Go 1.27 (`net/http` 標準ルーター、`database/sql`)
- **データベース**: PostgreSQL 17
- **公認外部パッケージ**:
  - `github.com/lib/pq` (PostgreSQL ドライバ)
  - `github.com/google/uuid` (アプリケーション層での UUID v7 生成)
  - `firebase.google.com/go/v4` (FCM Web Push 通知送信)
  - `golang.org/x/crypto`, `golang.org/x/oauth2` (パスワード bcrypt ハッシュ化・Google OAuth 2.0)

### フロントエンド (`frontend/`)
- **フレームワーク**: Nuxt 3 (SPA モード `ssr: false` / Vue 3 Composition API + TypeScript)
- **公認外部パッケージ**:
  - `marked` (Markdown パーサー)
  - `dompurify` (XSS サニタイズ)
  - `lucide-vue-next` (アイコンコンポーネント)
  - `firebase` (`firebase/messaging` による Web Push トークン登録・通知受信)

### MCP サーバー (`mcp/`)
- **言語・実行環境**: Node.js 22+ (TypeScript 5.7, ESM `"type": "module"`)
- **公認外部パッケージ**:
  - `@modelcontextprotocol/sdk` (Model Context Protocol 公式 SDK)
  - `zod` (ツール引数のスキーマ定義および実行時バリデーション)

---

## ディレクトリ構成

```text
.
├── README.md
├── AGENTS.md                        # エージェント開発規約・Git/PR ワークフロー規程
├── docker-compose.yml               # PostgreSQL 17 + Go API + Nuxt 3 Web + Mailpit
├── docs/
│   ├── specification.md             # システム全体確定仕様書
│   ├── er.md                        # Mermaid ER図・全29テーブル定義
│   ├── openapi.yaml                 # OpenAPI 3.0.3 API仕様書
│   ├── scrum_starter_guide.md       # スクラム実践スタートガイド（開発・見積・スプリント運用）
│   └── testing_lessons_and_guidelines.md # テスト品質保証ガイドライン・事後検証分析
├── backend/
│   ├── Dockerfile
│   ├── go.mod
│   ├── go.sum
│   ├── cmd/api/main.go              # APIサーバーエントリポイント
│   ├── migrations/
│   │   ├── 001_init.sql             # 初期スキーマDDL（全29テーブル・インデックス）
│   │   └── migrations.go            # 起動時マイグレーションランナー
│   └── internal/
│       ├── api/                     # ルーティング・CORS・認証/権限ミドルウェア・HTTPハンドラ
│       ├── domain/                  # ドメインモデル・ロールアップ集計・権限判定・工数精度検証
│       ├── fcm/                     # Firebase Cloud Messaging マルチキャスト送信
│       ├── mail/                    # 内蔵ダイレクトMX送信 & 外部SMTPリレー送信
│       ├── pkg/                     # UUID v7 生成・AES-256-GCM 暗号化・bcrypt・SHA-256
│       └── store/                   # PostgreSQL データストア実装
├── frontend/
│   ├── Dockerfile
│   ├── nuxt.config.ts               # Nuxt 3 SPA 設定 (ssr: false)
│   ├── package.json
│   ├── assets/css/main.css          # デザインシステム CSS 変数・コンポーネントスタイル
│   ├── types/polaris.ts             # 全エンティティ TypeScript 型定義
│   ├── utils/format.ts              # JST日時・小数第2位工数・ガント座標計算・安全Markdown描画
│   ├── composables/
│   │   ├── useApi.ts                # API クライアント・グローバル状態・トースト通知
│   │   └── useFcm.ts                # Firebase Cloud Messaging Web Push 連携
│   ├── components/
│   │   ├── ItemDetailModal.vue      # 課題/タスク詳細モーダル（タブ構成・マスク表示・統合添付・コメント）
│   │   └── TemplateBuilder.vue      # テンプレート＆1〜3カラムレイアウト・項目別権限ビルダー
│   ├── layouts/default.vue          # サイドバー・プロジェクト切替・通知ベル・アカウント設定
│   ├── pages/
│   │   ├── index.vue                # プロジェクト一覧・未参加プロジェクト削除時の確認ダイアログ
│   │   ├── setup.vue                # 初回1人目のシステム管理者セットアップ画面
│   │   ├── login.vue                # メール＋パスワード & Google OAuth ログイン・パスワード再設定申請
│   │   ├── admin.vue                # システム全体管理画面 (manage_system)
│   │   ├── auth/                    # 招待受諾・パスワード再設定・メール変更確認・Googleコールバック
│   │   └── projects/[id].vue        # プロジェクトワークスペース（カンバン・ガント・一覧・スプリント・設定）
│   └── tests/
│       ├── format.test.mjs          # フォーマット・計算系単体テスト
│       └── agile_scrum_ux.test.mjs  # アジャイル・スクラム・UX強化系単体テスト
└── mcp/                             # Model Context Protocol (MCP) サーバー
    ├── Dockerfile                   # node:22-alpine マルチステージ Dockerfile
    ├── package.json                 # @modelcontextprotocol/sdk, zod
    ├── README.md                    # MCP サーバー利用・クライアント設定ガイド（Claude Desktop, Cursor）
    ├── src/                         # REST クライアント・MCP ツール定義
    └── tests/                       # MCP 単体テスト
```

---

## 主な機能

1. **UUID v7 主キー & タイムゾーン管理**
   - すべてのテーブルの主キーはアプリケーション側で生成する **UUID v7**（時系列ソート可能）を使用。
   - 日時はすべて `TIMESTAMPTZ`（UTC）で保存し、フロントエンド表示および日付基準は JST（`Asia/Tokyo`）で統一。
2. **ハイブリッド初期管理者セットアップ & 招待制ユーザー管理**
   - 初回起動時（`users == 0`）は環境変数 `INIT_ADMIN_EMAIL` / `INIT_ADMIN_PASSWORD` による自動作成、または `/setup` 画面による初期セットアップをサポート。
   - 2人目以降は管理者による招待制（72時間有効トークン）。許可ドメイン制限（最大10件・サフィックス一致）に対応。
3. **3階層権限モデル & フィールド単位アクセス制御**
   - **システム権限**（`create_project`, `delete_project`, `manage_system`）：未参加プロジェクト削除時の安全確認フラグ（`confirm_non_member=true`）をサポート。
   - **プロジェクト権限**（`admin` > `editor` > `viewer`）：複数グループ所属時は最も強いロールを自動適用。
   - **フィールド権限**：テンプレート配置フィールドごとに全員公開／制限モード、対象ユーザー・グループ、オーナー特例（`allow_owner_view` / `allow_owner_edit`）を評価。非表示フィールドは API レスポンスから実データを除外し、画面上の配置枠を維持したまま `"🔒 閲覧権限がありません"` プレースホルダーを表示。
4. **課題・タスク階層と自動ロールアップ集計（工数・コスト予実・小数点第2位対応）**
   - 課題作成時の同名初期タスク同時作成オプション（`create_initial_task: true`）に対応。
   - 1日あたりの基準時間（`1.00`〜`24.00h`）、タスクの予定・実績工数（`planned_hours`, `actual_hours`）、およびタスクの予定・実績コスト（`planned_cost`, `actual_cost`）はすべて **小数点第2位まで** 対応。
   - 子タスクの予定／実績開始・終了日、予定／実績工数（h）、**予定／実績コスト**、加重平均進捗率（`planned_hours` または `planned_days * hours_per_day` 重み付け）、アサイン一覧を親課題へリアルタイム集計し、さらに **プロジェクトレベルでも予定／実績総コスト（`total_planned_cost`, `total_actual_cost`）を自動集計** 。
   - テンプレートの各項目（`title` を除く標準項目・カスタム項目）には閲覧・編集権限に加えて **項目非表示フラグ（`is_hidden`）** を設定可能。
5. **マトリクス・カンバン ＆ 予実2段バー対応ガントチャート**
   - **マトリクス・カンバン**：縦軸に課題スイムレーン、横軸にステータス（1〜10列）。セル内の直接タスク追加（`＋`）、横移動（ステータス変更）・縦移動（親課題変更）・セル内順序変更に対応。
   - **ガントチャート**：上段に予定バー（青＋進捗率）、下段に実績バー（緑）を2段表示し、どちらもドラッグ移動・左右端リサイズが可能。日付未設定タスクもタイムライン上のドラッグで直接バーを作成可能。
6. **DB チャンク方式ファイルストレージ（SHA-512 重複排除） & 自動ガベージコレクション**
   - 課題・タスク・コメントからの添付ファイルをすべて課題またはタスク直下に紐付け（上限100件／1ファイル最大1GB）。
   - **SHA-512** ハッシュによる重複排除（`file_blobs`）と 2MB 単位の分割保存（`file_chunks`）、参照カウント0時の自動GCを実装。

---

## 起動方法

詳細は **[クイックスタートガイド (`QUICKSTART.md`)](QUICKSTART.md)** をご覧ください。

### 1. クイックスタートスクリプトで一括起動する場合（推奨）

```bash
# 依存確認・.env初期化・Docker Compose起動をワンコマンドで実行
./quickstart.sh
```

手動で起動する場合：
```bash
cp .env.example .env
docker compose up -d --build
```

- Frontend Web (Nuxt 3 SPA): `http://localhost:3000`
- Backend API (Go): `http://localhost:8080`
- Mailpit (開発用メール確認 UI): `http://localhost:8025`
- PostgreSQL 17: `localhost:5432` (コンテナ内: `5432`)

### 2. ローカルで個別に起動する場合

#### バックエンド
```bash
cd backend
go build -o bin/polaris-api ./cmd/api
./bin/polaris-api
```

#### フロントエンド
```bash
cd frontend
npm install
npm run dev
```

---

## Google アカウント連携 (OAuth 2.0) の設定手順

Polaris では、メールアドレス／パスワードによる認証に加えて、Google アカウントを利用したシングルサインオン（SSO / OAuth 2.0）を標準でサポートしています。Google 連携を有効化するには、以下の手順で Google Cloud Console から認証情報を取得し、環境変数に設定してください。

### 1. Google Cloud Console でのプロジェクト準備
1. [Google Cloud Console](https://console.cloud.google.com/) にアクセスし、新しいプロジェクトを作成（または既存のプロジェクトを選択）します。

### 2. OAuth 同意画面の設定
1. 左側ナビゲーションメニューから **「APIとサービス」** ➔ **「OAuth 同意画面」** を開きます。
2. **User Type** を選択します：
   - 社内・特定組織のみで利用する場合: **「内部」**
   - 一般公開または組織外の Google アカウントも許可する場合: **「外部」**
3. アプリ名（例: `Polaris Project Management`）、ユーザーサポートメール、開発者の連絡先情報を入力して保存します。
4. スコープ設定では、標準の `.../auth/userinfo.email`、`.../auth/userinfo.profile`、`openid` が含まれていることを確認します。

### 3. OAuth クライアント ID の発行
1. **「APIとサービス」** ➔ **「認証情報」** を開き、上部の **「認証情報を作成」** ➔ **「OAuth クライアント ID」** を選択します。
2. 各項目を以下のように入力します：
   - **アプリケーションの種類**: `ウェブ アプリケーション`
   - **名前**: 任意の識別名（例: `Polaris Web Client`）
   - **承認済みの JavaScript 生成元**:
     - `http://localhost:3000`（本番運用の場合は公開ドメイン、例: `https://polaris.yourdomain.com`）
   - **承認済みのリダイレクト URI**:
     - `http://localhost:3000/auth/google/callback`（本番運用の場合は `https://polaris.yourdomain.com/auth/google/callback`）
3. **「作成」** をクリックすると、「クライアント ID」および「クライアント シークレット」がポップアップ表示されます。

### 4. Polaris への環境変数設定
発行されたキーを `.env` ファイル（または Docker Compose の環境変数）に設定します：

```env
# Google OAuth 2.0 連携
GOOGLE_CLIENT_ID=xxxxxxxxxxxx-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=GOCSPX-xxxxxxxxxxxxxxxxxxxxxxxx
GOOGLE_REDIRECT_URL=http://localhost:3000/auth/google/callback
```

設定後にコンテナまたは API サーバーを再起動すると、ログイン画面および初期管理者登録画面に **「Google でログイン」** ボタンが自動的に表示されます。

---

## 環境変数リファレンス

システム全体で利用可能な環境変数の一覧です。`.env` ファイルに定義することで、Docker Compose 起動時およびローカル実行時に自動適用されます。

### バックエンド API (`backend/`)

| 環境変数名 | 必須 | デフォルト値 | 説明 |
| :--- | :---: | :---: | :--- |
| `PORT` | 任意 | `8080` | API サーバーがリッスンする HTTP ポート番号 |
| `DATABASE_URL` | 任意 | `postgres://polaris:polaris@localhost:5432/polaris?sslmode=disable` | PostgreSQL 接続文字列 |
| `APP_BASE_URL` | 任意 | `http://localhost:3000` | フロントエンド Web アプリケーションのベース URL（メール内リンクやリダイレクト生成に使用） |
| `CORS_ALLOWED_ORIGIN` | 任意 | `http://localhost:3000` | CORS を許可するオリジン（通常は `APP_BASE_URL` と同一） |
| `ENCRYPTION_KEY` | 任意* | `0123456789abcdef0123456789abcdef` | 機密データ暗号化キー（AES-256-GCM 用。必ず半角英数字 32 文字 / 32 バイト）。*本番運用時は変更必須 |
| `INIT_ADMIN_EMAIL` | 任意 | *(空欄)* | 初回起動時に自動作成する初期システム管理者のメールアドレス |
| `INIT_ADMIN_PASSWORD` | 任意 | *(空欄)* | 初回起動時に自動作成する初期システム管理者のパスワード |
| `INIT_ADMIN_NAME` | 任意 | `System Admin` | 初期システム管理者の表示名 |
| `GOOGLE_CLIENT_ID` | 任意 | *(空欄)* | Google OAuth 2.0 クライアント ID |
| `GOOGLE_CLIENT_SECRET` | 任意 | *(空欄)* | Google OAuth 2.0 クライアント シークレット |
| `GOOGLE_REDIRECT_URL` | 任意 | `http://localhost:3000/auth/google/callback` | Google OAuth 2.0 リダイレクト先コールバック URL |
| `FIREBASE_CREDENTIALS_FILE` | 任意 | *(空欄)* | Firebase Admin サービスアカウント秘密鍵 JSON ファイルへのパス（Web Push 送信用） |

### データベース & インフラ (`docker-compose.yml`)

| 環境変数名 | 必須 | デフォルト値 | 説明 |
| :--- | :---: | :---: | :--- |
| `POSTGRES_USER` | 任意 | `polaris` | PostgreSQL データベースの接続ユーザー名 |
| `POSTGRES_PASSWORD` | 任意 | `polaris` | PostgreSQL データベースの接続パスワード |
| `POSTGRES_DB` | 任意 | `polaris` | 作成・接続するデータベース名 |
| `POSTGRES_PORT` | 任意 | `5432` | ホスト側へマッピングする PostgreSQL ポート番号 |
| `API_PORT` | 任意 | `8080` | ホスト側へマッピングする バックエンド API ポート番号 |
| `WEB_PORT` | 任意 | `3000` | ホスト側へマッピングする フロントエンド Web ポート番号 |
| `MAILPIT_UI_PORT` | 任意 | `8025` | Mailpit 開発用メールプレビュー Web UI ポート番号 |
| `MAILPIT_SMTP_PORT` | 任意 | `1025` | Mailpit 開発用 SMTP サーバー受付ポート番号 |

### フロントエンド (`frontend/`)

| 環境変数名 | 必須 | デフォルト値 | 説明 |
| :--- | :---: | :---: | :--- |
| `NUXT_PUBLIC_API_BASE_URL` | 任意 | `http://localhost:8080/api/v1` | ブラウザから呼び出すバックエンド API のベース URL |
| `NUXT_PUBLIC_FIREBASE_API_KEY` | 任意 | *(空欄)* | Firebase Web Push 用 API キー |
| `NUXT_PUBLIC_FIREBASE_AUTH_DOMAIN` | 任意 | *(空欄)* | Firebase Web Push 用認証ドメイン |
| `NUXT_PUBLIC_FIREBASE_PROJECT_ID` | 任意 | *(空欄)* | Firebase プロジェクト ID |
| `NUXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID` | 任意 | *(空欄)* | Firebase FCM 送信者 ID |
| `NUXT_PUBLIC_FIREBASE_APP_ID` | 任意 | *(空欄)* | Firebase アプリケーション ID |
| `NUXT_PUBLIC_FIREBASE_VAPID_KEY` | 任意 | *(空欄)* | Firebase Web Push 通知受信用の VAPID 公開鍵 |

### MCP サーバー (`mcp/`)

| 環境変数名 | 必須 | デフォルト値 | 説明 |
| :--- | :---: | :---: | :--- |
| `POLARIS_BASE_URL` | 任意 | `http://localhost:8080/api/v1` | MCP サーバーが接続する Polaris API のベース URL |
| `POLARIS_API_TOKEN` | 条件付 | *(空欄)* | 個人用アクセストークン（PAT）。Google 連携ユーザーや推奨認証方式 |
| `POLARIS_EMAIL` | 条件付 | *(空欄)* | 自動ログイン用メールアドレス（`POLARIS_API_TOKEN` 未設定時に使用） |
| `POLARIS_PASSWORD` | 条件付 | *(空欄)* | 自動ログイン用パスワード（`POLARIS_API_TOKEN` 未設定時に使用） |

---

## テストの実行方法

### バックエンド単体テスト & Linter
```bash
go test -C backend -v ./...
go vet -C backend ./...
```

### フロントエンド単体テスト
```bash
cd frontend
npm test
```

### MCP サーバー単体テスト
```bash
cd mcp
npm test
```

---

## 開発・コントリビューション規約 (Git / PR ワークフロー)

プロジェクトへの変更・コミットを行う際は、以下のワークフロー規程を厳格に遵守してください。詳細なエージェント役割分担および規約は [`AGENTS.md`](AGENTS.md) を参照してください。

1. **`main` ブランチ直接コミット・プッシュの禁止**:
   - 必ず目的別のフィーチャーブランチ（`feat/*`, `fix/*`, `docs/*`, `chore/*` 等）を作成して作業を行ってください。
2. **Pull Request 必須**:
   - 作業ブランチをプッシュ後、GitHub CLI (`gh pr create`) または Web UI から Pull Request を作成してください。
3. **レビュー・承認プロセス**:
   - 単体テスト・ビルドの完全通過、テスターペルソナ受入検証、およびレビュアーの承認（`APPROVE`）を経てからマージ（`gh pr merge --merge`）を実施してください。

---

## ライセンス

本プロジェクトは [MIT License](LICENSE) の下で公開されています。

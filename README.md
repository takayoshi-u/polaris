# Polaris — プロジェクト管理 Web アプリケーション

**Polaris** は、「プロジェクト ＞ 課題（親） ＞ タスク（子）」の明確な階層構造、フィールド単位のきめ細かな権限・テンプレートエンジン、課題スイムレーン型マトリクス・カンバンボード、予実2段バー対応のガントチャートを備えたプロジェクト管理 Web アプリケーションです。

## ドキュメント一覧

システムの詳細仕様・データベース設計・API仕様は以下のドキュメントを参照してください。

- **[確定仕様書 (`docs/specification.md`)](docs/specification.md)** — システム全体のドメインルール、権限モデル、画面・機能仕様
- **[ER図・テーブル定義書 (`docs/er.md`)](docs/er.md)** — PostgreSQL 17 データベースの Mermaid ER図および全 29 テーブルの定義（全主キー UUID v7）
- **[OpenAPI 3.0.3 仕様書 (`docs/openapi.yaml`)](docs/openapi.yaml)** — バックエンド REST API（`/api/v1`）の全エンドポイント・リクエスト／レスポンススキーマ定義
- **[エージェント開発規約・PR運用規程 (`AGENTS.md`)](AGENTS.md)** — Git ブランチ戦略、Pull Request 運用ルール、エージェントチーム役割分担

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
│   └── openapi.yaml                 # OpenAPI 3.0.3 API仕様書
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
└── frontend/
    ├── Dockerfile
    ├── nuxt.config.ts               # Nuxt 3 SPA 設定 (ssr: false)
    ├── package.json
    ├── assets/css/main.css          # デザインシステム CSS 変数・コンポーネントスタイル
    ├── types/polaris.ts             # 全エンティティ TypeScript 型定義
    ├── utils/format.ts              # JST日時・小数第2位工数・ガント座標計算・安全Markdown描画
    ├── composables/
    │   ├── useApi.ts                # API クライアント・グローバル状態・トースト通知
    │   └── useFcm.ts                # Firebase Cloud Messaging Web Push 連携
    ├── components/
    │   ├── ItemDetailModal.vue      # 課題/タスク詳細モーダル（1〜3カラム・マスク表示・統合添付・コメント）
    │   └── TemplateBuilder.vue      # テンプレート＆1〜3カラムレイアウト・項目別権限ビルダー
    ├── layouts/default.vue          # サイドバー・プロジェクト切替・通知ベル・アカウント設定
    ├── pages/
    │   ├── index.vue                # プロジェクト一覧・未参加プロジェクト削除時の確認ダイアログ
    │   ├── setup.vue                # 初回1人目のシステム管理者セットアップ画面
    │   ├── login.vue                # メール＋パスワード & Google OAuth ログイン・パスワード再設定申請
    │   ├── admin.vue                # システム全体管理画面 (manage_system)
    │   ├── auth/                    # 招待受諾・パスワード再設定・メール変更確認・Googleコールバック
    │   └── projects/[id].vue        # プロジェクトワークスペース（カンバン・ガント・一覧・設定）
    └── tests/
        └── format.test.mjs          # フロントエンド単体テスト
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
   - 1日あたりの基準時間（`1.00`〜`24.00h`）、タスクの予定・実績工数（`planned_hours`, `actual_hours`）、およびタスクの予定・実績コスト（`planned_cost`, `actual_cost`）はすべて**小数点第2位まで**対応。
   - 子タスクの予定／実績開始・終了日、予定／実績工数（h）、**予定／実績コスト**、加重平均進捗率（`planned_hours` または `planned_days * hours_per_day` 重み付け）、アサイン一覧を親課題へリアルタイム集計し、さらに**プロジェクトレベルでも予定／実績総コスト（`total_planned_cost`, `total_actual_cost`）を自動集計**。
   - テンプレートの各項目（`title` を除く標準項目・カスタム項目）には閲覧・編集権限に加えて**項目非表示フラグ（`is_hidden`）**を設定可能。
5. **マトリクス・カンバン ＆ 予実2段バー対応ガントチャート**
   - **マトリクス・カンバン**：縦軸に課題スイムレーン、横軸にステータス（1〜10列）。横移動（ステータス変更）・縦移動（親課題変更）・セル内順序変更に対応。
   - **ガントチャート**：上段に予定バー（青＋進捗率）、下段に実績バー（緑）を2段表示し、どちらもドラッグ移動・左右端リサイズが可能。日付未設定タスクもタイムライン上のドラッグで直接バーを作成可能。
6. **DB チャンク方式ファイルストレージ（SHA-512 重複排除） & 自動ガベージコレクション**
   - 課題・タスク・コメントからの添付ファイルをすべて課題またはタスク直下に紐付け（上限100件／1ファイル最大1GB）。
   - **SHA-512** ハッシュによる重複排除（`file_blobs`）と 2MB 単位の分割保存（`file_chunks`）、参照カウント0時の自動GCを実装。

---

## 起動方法

### 1. Docker Compose で一括起動する場合

```bash
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

## テストの実行方法

### バックエンド単体テスト & Linter
```bash
go test -C backend -v ./...
go vet -C backend ./...
```

### フロントエンド単体テスト
```bash
node --test frontend/tests/format.test.mjs
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


<template>
  <div
    style="
      position: fixed;
      inset: 0;
      z-index: 1000;
      background: rgba(0, 0, 0, 0.45);
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 16px;
    "
    @click.self="$emit('close')"
  >
    <div class="card" style="width: 100%; max-width: 580px; padding: 20px; display: flex; flex-direction: column; gap: 16px;">
      <!-- Modal Header -->
      <div style="display: flex; align-items: center; justify-content: space-between;">
        <h3 style="font-size: 16px; font-weight: 700; display: flex; align-items: center; gap: 6px; margin: 0;">
          <span>📥 CSV タスク一括インポート</span>
        </h3>
        <button type="button" class="btn btn-ghost btn-sm" @click="$emit('close')">✕</button>
      </div>

      <!-- Result View (After Upload) -->
      <div v-if="importResult" style="display: flex; flex-direction: column; gap: 14px;">
        <div
          :style="{
            padding: '14px',
            borderRadius: '8px',
            background: importResult.imported_tasks > 0 ? '#dcfce7' : '#fee2e2',
            border: `1px solid ${importResult.imported_tasks > 0 ? '#86efac' : '#fca5a5'}`
          }"
        >
          <div style="font-size: 14px; font-weight: 700;" :style="{ color: importResult.imported_tasks > 0 ? '#15803d' : '#b91c1c' }">
            {{ importResult.imported_tasks > 0 ? `✓ ${importResult.imported_tasks} 件のタスクをインポートしました` : 'インポートに失敗しました' }}
          </div>
          <p v-if="importResult.errors && importResult.errors.length > 0" style="margin: 6px 0 0 0; font-size: 12px; color: #b91c1c;">
            一部の行でエラーまたはスキップが発生しました ({{ importResult.errors.length }}件):
          </p>
        </div>

        <!-- Errors / Warnings List -->
        <div
          v-if="importResult.errors && importResult.errors.length > 0"
          style="max-height: 180px; overflow-y: auto; background: var(--bg-input); border: 1px solid var(--border-color); border-radius: 6px; padding: 8px 12px; font-size: 12px; font-family: monospace; display: flex; flex-direction: column; gap: 4px;"
        >
          <div v-for="(err, idx) in importResult.errors" :key="idx" style="color: var(--danger);">
            • {{ err }}
          </div>
        </div>

        <div style="display: flex; justify-content: flex-end; gap: 8px;">
          <button type="button" class="btn btn-primary" @click="finishImport">
            完了して閉じる
          </button>
        </div>
      </div>

      <!-- Upload & Preview Form -->
      <div v-else style="display: flex; flex-direction: column; gap: 14px;">
        <p style="font-size: 12px; color: var(--text-muted); margin: 0;">
          エクスポートしたCSV形式に準拠したタスク一覧ファイル（UTF-8 CSV）を選択またはドロップしてください。
        </p>

        <!-- Drag & Drop File Zone -->
        <div
          class="file-drop-zone"
          :class="{ active: isDragging }"
          style="
            border: 2px dashed var(--border-color);
            border-radius: 8px;
            padding: 24px;
            text-align: center;
            cursor: pointer;
            background: isDragging ? 'var(--bg-elevated)' : 'var(--bg-card)';
            display: flex;
            flex-direction: column;
            align-items: center;
            gap: 8px;
          "
          @dragover.prevent="isDragging = true"
          @dragleave.prevent="isDragging = false"
          @drop.prevent="onFileDrop"
          @click="triggerFileInput"
        >
          <input
            ref="fileInputRef"
            type="file"
            accept=".csv,text/csv"
            style="display: none;"
            @change="onFileSelected"
          />
          <span style="font-size: 28px;">📄</span>
          <div v-if="selectedFile" style="font-weight: 600; font-size: 13px; color: var(--primary);">
            {{ selectedFile.name }} ({{ formatFileSize(selectedFile.size) }})
          </div>
          <div v-else style="font-size: 13px; color: var(--text-secondary);">
            ここに CSV ファイルをドラッグ＆ドロップ、またはクリックしてファイルを選択
          </div>
        </div>

        <!-- Parse Preview Section -->
        <div v-if="previewHeaders.length > 0" style="display: flex; flex-direction: column; gap: 6px;">
          <div style="font-size: 12px; font-weight: 700; color: var(--text-muted);">
            CSV プレビュー（最初の {{ previewRows.length }} 行）:
          </div>

          <div style="overflow-x: auto; border: 1px solid var(--border-color); border-radius: 6px; max-height: 160px;">
            <table style="width: 100%; border-collapse: collapse; font-size: 11px;">
              <thead>
                <tr style="background: var(--bg-elevated); border-bottom: 1px solid var(--border-color);">
                  <th v-for="(h, idx) in previewHeaders" :key="idx" style="padding: 4px 8px; text-align: left; white-space: nowrap;">
                    {{ h }}
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, rIdx) in previewRows" :key="rIdx" style="border-bottom: 1px solid var(--border-color);">
                  <td v-for="(col, cIdx) in row" :key="cIdx" style="padding: 4px 8px; white-space: nowrap; max-width: 140px; overflow: hidden; text-overflow: ellipsis;">
                    {{ col }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Actions -->
        <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 6px;">
          <button type="button" class="btn btn-secondary" @click="$emit('close')">
            キャンセル
          </button>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="!selectedFile || isUploading"
            @click="executeImport"
          >
            {{ isUploading ? 'インポート中...' : 'インポートを実行' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { CSVImportResult } from '~/types/polaris'
import { formatFileSize } from '~/utils/format'

const props = defineProps<{
  projectId: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'imported'): void
}>()

const { apiFetch, notifyToast } = useApi()

const fileInputRef = ref<HTMLInputElement | null>(null)
const isDragging = ref(false)
const selectedFile = ref<File | null>(null)
const previewHeaders = ref<string[]>([])
const previewRows = ref<string[][]>([])
const isUploading = ref(false)
const importResult = ref<CSVImportResult | null>(null)

function triggerFileInput() {
  fileInputRef.value?.click()
}

function onFileSelected(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (file) {
    handleFile(file)
  }
}

function onFileDrop(event: DragEvent) {
  isDragging.value = false
  const file = event.dataTransfer?.files?.[0]
  if (file) {
    handleFile(file)
  }
}

function handleFile(file: File) {
  selectedFile.value = file
  parsePreview(file)
}

function parsePreview(file: File) {
  const reader = new FileReader()
  reader.onload = (e) => {
    const text = (e.target?.result as string) || ''
    // Strip BOM if present
    const clean = text.replace(/^\uFEFF/, '')
    const lines = clean.split(/\r?\n/).filter(line => line.trim().length > 0)
    if (lines.length > 0) {
      previewHeaders.value = parseCSVLine(lines[0])
      previewRows.value = lines.slice(1, 4).map(parseCSVLine)
    } else {
      previewHeaders.value = []
      previewRows.value = []
    }
  }
  reader.readAsText(file, 'utf-8')
}

function parseCSVLine(line: string): string[] {
  const result: string[] = []
  let cur = ''
  let inQuotes = false

  for (let i = 0; i < line.length; i++) {
    const char = line[i]
    if (char === '"') {
      if (inQuotes && line[i + 1] === '"') {
        cur += '"'
        i++
      } else {
        inQuotes = !inQuotes
      }
    } else if (char === ',' && !inQuotes) {
      result.push(cur.trim())
      cur = ''
    } else {
      cur += char
    }
  }
  result.push(cur.trim())
  return result
}

async function executeImport() {
  if (!selectedFile.value) return
  isUploading.value = true

  const formData = new FormData()
  formData.append('file', selectedFile.value)

  try {
    const res = await apiFetch<CSVImportResult>(`/projects/${props.projectId}/import/csv`, {
      method: 'POST',
      body: formData
    })
    importResult.value = res
    if (res.imported_tasks > 0) {
      notifyToast(`${res.imported_tasks}件のタスクをインポートしました`, 'success')
    }
  } catch (e: any) {
    notifyToast(e.message || 'CSVインポートに失敗しました', 'error')
  } finally {
    isUploading.value = false
  }
}

function finishImport() {
  emit('imported')
  emit('close')
}
</script>

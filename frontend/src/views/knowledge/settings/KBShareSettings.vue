<template>
  <div class="share-to-space-panel">
    <div class="share-panel-header">
      <div class="share-panel-header-row">
        <div class="share-panel-titlewrap">
          <h2 class="share-panel-title">{{ $t('organization.share.title') }}</h2>
          <t-popup placement="bottom-start" trigger="hover" overlay-class-name="wk-popover share-hint-popup-overlay"
            :overlay-inner-style="shareHintPopupInnerStyle">
            <button type="button" class="share-hint-trigger-btn" :aria-label="$t('knowledgeEditor.share.hintTitle')"
              :title="$t('knowledgeEditor.share.hintTitle')">
              <t-icon name="info-circle" size="16px" />
            </button>
            <template #content>
              <div class="share-hint-popover">
                <p class="share-hint-title">{{ $t('knowledgeEditor.share.hintTitle') }}</p>
                <p class="share-hint-desc">{{ $t('knowledgeEditor.share.tip1') }}</p>
                <p class="share-hint-desc">{{ $t('knowledgeEditor.share.tip2') }}</p>
              </div>
            </template>
          </t-popup>
        </div>
      </div>
      <p class="share-panel-desc">{{ $t('organization.share.descriptionUnified') }}</p>
    </div>

    <div class="share-panel-list-wrap">
      <div class="share-panel-list-header">
        <div class="share-panel-titlewrap">
          <span class="share-panel-list-title">{{ $t('organization.share.sharedTo') }}</span>
          <span class="share-panel-count-badge">{{ filteredRows.length }}</span>
        </div>
        <div class="share-panel-actions">
          <div class="share-panel-search">
            <t-input v-model="searchQuery" size="small" :placeholder="$t('organization.share.searchUnifiedPlaceholder')" clearable>
              <template #prefix-icon><t-icon name="search" /></template>
            </t-input>
          </div>
          <t-popup v-if="canShare" v-model="addPopupVisible" trigger="click" placement="bottom-end" destroy-on-close
            overlay-class-name="share-add-popup-overlay">
            <t-button theme="primary" variant="outline" shape="square" size="small" class="share-panel-add-btn"
              :title="$t('knowledgeEditor.share.addShare')" :aria-label="$t('knowledgeEditor.share.addShare')">
              <template #icon><t-icon name="add" /></template>
            </t-button>
            <template #content>
              <div class="share-add-popup-inner share-add-popup-wide" @click.stop>
                <div class="member-invite-popup-title">{{ $t('organization.share.addShareDialogTitleUnified') }}</div>

                <div class="share-target-switch">
                  <button type="button" :class="{ active: shareTargetType === 'space' }" @click="shareTargetType = 'space'">
                    {{ $t('organization.share.targetSpace') }}
                  </button>
                  <button type="button" :class="{ active: shareTargetType === 'user' }" @click="shareTargetType = 'user'">
                    {{ $t('organization.share.targetUser') }}
                  </button>
                </div>

                <div class="org-upgrade-fields">
                  <div v-if="shareTargetType === 'space'" class="org-upgrade-field">
                    <label class="org-upgrade-field-label">{{ $t('organization.share.selectOrg') }}</label>
                    <ShareToSpaceOrgSelect v-model="selectedOrgId" :organizations="availableOrganizations"
                      :loading="loadingOrgs" />
                  </div>

                  <div v-else class="org-upgrade-field">
                    <label class="org-upgrade-field-label">{{ $t('organization.share.selectUser') }}</label>
                    <t-input v-model="userQuery" :placeholder="$t('organization.share.searchUserPlaceholder')" clearable />
                    <div v-if="searchingUsers" class="user-share-search-status">
                      <t-loading size="small" /> {{ $t('organization.share.searchingUsers') }}
                    </div>
                    <div v-else-if="userQuery.trim().length >= 2 && userCandidates.length === 0"
                      class="user-share-search-status">
                      {{ $t('organization.share.noUserCandidates') }}
                    </div>
                    <div v-if="userCandidates.length > 0" class="user-share-candidates">
                      <button v-for="candidate in userCandidates" :key="candidate.id" type="button"
                        class="user-share-candidate" :class="{ selected: selectedUserId === candidate.id }"
                        @click="selectedUserId = candidate.id">
                        <span class="user-share-candidate-main">
                          <strong>{{ candidate.username || candidate.email }}</strong>
                          <small>{{ candidate.email }}</small>
                        </span>
                        <t-icon v-if="selectedUserId === candidate.id" name="check" />
                      </button>
                    </div>
                    <p class="member-form-hint">{{ $t('organization.share.userShareHint') }}</p>
                  </div>

                  <div class="org-upgrade-field org-upgrade-field--last">
                    <label class="org-upgrade-field-label">{{ $t('organization.share.permission') }}</label>
                    <t-select v-model="selectedPermission" size="medium">
                      <t-option value="viewer" :label="$t('organization.share.permissionReadonly')" />
                      <t-option value="editor" :label="$t('organization.share.permissionEditable')" />
                    </t-select>
                    <p class="member-form-hint">{{ $t('organization.share.permissionTip') }}</p>
                  </div>
                </div>

                <div class="invite-popup-footer">
                  <t-button variant="outline" :disabled="submitting" @click="closeAddPopup">
                    {{ $t('common.cancel') }}
                  </t-button>
                  <t-button theme="primary" :loading="submitting" :disabled="!canSubmitShare" @click="handleShare">
                    {{ $t('knowledgeEditor.share.addShare') }}
                  </t-button>
                </div>
              </div>
            </template>
          </t-popup>
        </div>
      </div>

      <div v-if="loadingShares && rows.length === 0" class="share-panel-loading">
        <t-loading size="small" />
        <span>{{ $t('organization.share.loading') }}</span>
      </div>
      <div v-else-if="filteredRows.length === 0" class="share-panel-empty">
        <t-empty :description="searchQuery.trim()
          ? $t('organization.share.emptyUnifiedSearch', { q: searchQuery })
          : $t('organization.share.noUnifiedShares')" />
      </div>
      <div v-else class="share-panel-table-shell">
        <t-table row-key="rowKey" :data="filteredRows" :columns="shareColumns" size="medium" hover stripe
          :loading="loadingShares">
          <template #target="{ row }">
            <div class="share-space-cell">
              <span class="share-space-name">
                <SpaceAvatar v-if="row.kind === 'space'" :name="row.name || ''"
                  :avatar="getOrgForShare(row.organizationId)?.avatar" size="small" />
                <span v-else class="user-share-avatar">{{ userInitial(row.name) }}</span>
                <span class="share-space-name-text">{{ row.name }}</span>
              </span>
              <span class="share-space-meta">
                {{ row.kind === 'space'
                  ? $t('organization.share.targetSpace')
                  : $t('organization.share.targetUser') }}
                <template v-if="row.subtitle"> · {{ row.subtitle }}</template>
              </span>
            </div>
          </template>
          <template #permission="{ row }">
            <div class="share-permission-cell">
              <t-select v-if="canShare" :value="row.permission" size="small" class="share-permission-select"
                @change="(val: string) => handleUpdatePermission(row, val)">
                <t-option value="viewer" :label="$t('organization.share.permissionReadonly')" />
                <t-option value="editor" :label="$t('organization.share.permissionEditable')" />
              </t-select>
              <t-tag v-else size="small" :theme="row.permission === 'editor' ? 'warning' : 'default'" variant="light">
                {{ permissionLabel(row.permission) }}
              </t-tag>
            </div>
          </template>
          <template #created_at="{ row }">{{ formatShareDate(row.createdAt) }}</template>
          <template #actions="{ row }">
            <div v-if="canShare" class="share-table-actions">
              <t-popconfirm :content="$t('organization.share.unshareTargetConfirm', { name: row.name })"
                :confirm-btn="{ content: $t('common.confirm'), theme: 'danger' }"
                :cancel-btn="{ content: $t('common.cancel') }" placement="left" @confirm="handleUnshare(row)">
                <t-tooltip :content="$t('organization.share.unshareAction')" placement="top">
                  <t-button theme="danger" shape="square" variant="text" size="small" @click.stop>
                    <template #icon><t-icon name="delete" /></template>
                  </t-button>
                </t-tooltip>
              </t-popconfirm>
            </div>
          </template>
        </t-table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import { useOrganizationStore } from '@/stores/organization'
import {
  listKBShares,
  listKBUserShares,
  shareKnowledgeBaseToUser,
  searchKBUserShareCandidates,
  updateUserSharePermission,
  removeUserShare,
} from '@/api/organization'
import type {
  KnowledgeBaseShare,
  KnowledgeBaseUserShare,
  UserShareCandidate,
} from '@/api/organization'
import SpaceAvatar from '@/components/SpaceAvatar.vue'
import ShareToSpaceOrgSelect from '@/components/ShareToSpaceOrgSelect.vue'

const { t } = useI18n()
const orgStore = useOrganizationStore()

interface Props {
  kbId: string
  canShare?: boolean
}

type ShareRow = {
  rowKey: string
  kind: 'space' | 'user'
  id: string
  name: string
  subtitle?: string
  permission: 'viewer' | 'editor'
  createdAt: string
  organizationId?: string
}

const props = withDefaults(defineProps<Props>(), { canShare: false })

const shareHintPopupInnerStyle = {
  boxSizing: 'border-box' as const,
  padding: '0',
  width: 'min(400px, calc(100vw - 24px))',
  maxWidth: 'min(400px, calc(100vw - 24px))',
  maxHeight: 'min(280px, 65vh)',
  overflow: 'hidden',
}

const loadingOrgs = ref(false)
const loadingShares = ref(false)
const submitting = ref(false)
const addPopupVisible = ref(false)
const searchQuery = ref('')
const selectedOrgId = ref('')
const selectedPermission = ref<'viewer' | 'editor'>('viewer')
const shareTargetType = ref<'space' | 'user'>('space')
const userQuery = ref('')
const selectedUserId = ref('')
const userCandidates = ref<UserShareCandidate[]>([])
const searchingUsers = ref(false)
const shares = ref<(KnowledgeBaseShare & { organization_name?: string })[]>([])
const userShares = ref<KnowledgeBaseUserShare[]>([])
let searchTimer: ReturnType<typeof setTimeout> | null = null
let searchGeneration = 0

function getOrgForShare(organizationId?: string) {
  if (!organizationId) return undefined
  return orgStore.organizations.find(o => o.id === organizationId)
}

const availableOrganizations = computed(() => {
  const sharedOrgIds = new Set(shares.value.map(s => s.organization_id))
  return orgStore.organizations.filter(
    (org) =>
      !sharedOrgIds.has(org.id) &&
      (org.is_owner === true || org.my_role === 'admin' || org.my_role === 'editor')
  )
})

const rows = computed<ShareRow[]>(() => [
  ...shares.value.map((share) => ({
    rowKey: `space:${share.id}`,
    kind: 'space' as const,
    id: share.id,
    name: share.organization_name || share.organization_id,
    subtitle: share.shared_by_username
      ? `${t('organization.share.sharedFrom')} ${share.shared_by_username}`
      : undefined,
    permission: (share.permission === 'editor' || share.permission === 'admin' ? 'editor' : 'viewer') as 'viewer' | 'editor',
    createdAt: share.created_at,
    organizationId: share.organization_id,
  })),
  ...userShares.value.map((share) => ({
    rowKey: `user:${share.id}`,
    kind: 'user' as const,
    id: share.id,
    name: share.target_username || share.target_email || share.target_user_id,
    subtitle: share.target_email && share.target_email !== share.target_username ? share.target_email : undefined,
    permission: share.permission,
    createdAt: share.created_at,
  })),
])

const filteredRows = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  if (!query) return rows.value
  return rows.value.filter((row) =>
    [row.name, row.subtitle, permissionLabel(row.permission)]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
      .includes(query)
  )
})

const shareColumns = computed(() => {
  const cols = [
    { colKey: 'target', title: t('organization.share.columns.target'), ellipsis: true, minWidth: 190 },
    { colKey: 'permission', title: t('organization.share.columns.permission'), width: 132 },
    { colKey: 'created_at', title: t('organization.share.columns.sharedAt'), width: 154 },
  ]
  if (props.canShare) {
    cols.push({ colKey: 'actions', title: t('organization.share.columns.operations'), width: 72, align: 'left' } as typeof cols[number])
  }
  return cols
})

const canSubmitShare = computed(() =>
  shareTargetType.value === 'space' ? !!selectedOrgId.value : !!selectedUserId.value
)

function permissionLabel(permission: string) {
  return permission === 'editor'
    ? t('organization.share.permissionEditable')
    : t('organization.share.permissionReadonly')
}

function formatShareDate(dateStr?: string) {
  if (!dateStr) return '—'
  const date = new Date(dateStr)
  if (Number.isNaN(date.getTime())) return dateStr
  return date.toLocaleDateString(undefined, { year: 'numeric', month: '2-digit', day: '2-digit' })
}

function userInitial(name?: string) {
  const value = (name || '?').trim()
  return value.slice(0, 1).toUpperCase()
}

async function loadOrganizations() {
  loadingOrgs.value = true
  try {
    await orgStore.fetchOrganizations()
  } finally {
    loadingOrgs.value = false
  }
}

async function loadShares() {
  if (!props.kbId) return
  loadingShares.value = true
  try {
    const [orgResult, userResult] = await Promise.all([
      listKBShares(props.kbId),
      listKBUserShares(props.kbId),
    ])
    if (orgResult.success && orgResult.data) {
      const sharesData = (orgResult.data as { shares?: KnowledgeBaseShare[] }).shares || orgResult.data
      const sharesList = Array.isArray(sharesData) ? sharesData : []
      shares.value = sharesList.map((share: KnowledgeBaseShare) => ({
        ...share,
        organization_name:
          share.organization_name ||
          orgStore.organizations.find(o => o.id === share.organization_id)?.name ||
          share.organization_id,
      }))
    }
    if (userResult.success && userResult.data) {
      userShares.value = userResult.data.shares || []
    }
  } catch (e) {
    console.error('Failed to load shares:', e)
  } finally {
    loadingShares.value = false
  }
}

function resetUserSearch() {
  userQuery.value = ''
  selectedUserId.value = ''
  userCandidates.value = []
  searchingUsers.value = false
  searchGeneration += 1
  if (searchTimer) {
    clearTimeout(searchTimer)
    searchTimer = null
  }
}

function closeAddPopup() {
  addPopupVisible.value = false
  selectedOrgId.value = ''
  selectedPermission.value = 'viewer'
  shareTargetType.value = 'space'
  resetUserSearch()
}

async function handleShare() {
  if (!canSubmitShare.value) return
  submitting.value = true
  try {
    const result = shareTargetType.value === 'space'
      ? await orgStore.shareKnowledgeBase(props.kbId, {
          organization_id: selectedOrgId.value,
          permission: selectedPermission.value,
        })
      : await shareKnowledgeBaseToUser(props.kbId, {
          user_id: selectedUserId.value,
          permission: selectedPermission.value,
        })

    if (result.success) {
      MessagePlugin.success(t('organization.share.shareSuccess'))
      closeAddPopup()
      await loadShares()
    } else {
      MessagePlugin.error(result.message || t('organization.share.shareFailed'))
    }
  } catch (e: unknown) {
    MessagePlugin.error(e instanceof Error ? e.message : t('organization.share.shareFailed'))
  } finally {
    submitting.value = false
  }
}

async function handleUpdatePermission(row: ShareRow, newPermission: string) {
  const permission = newPermission as 'viewer' | 'editor'
  if (row.permission === permission) return
  try {
    const result = row.kind === 'space'
      ? await orgStore.changeKnowledgeBaseSharePermission(props.kbId, row.id, { permission })
      : await updateUserSharePermission(props.kbId, row.id, { permission })
    if (result.success) {
      MessagePlugin.success(t('organization.roleUpdated'))
      await loadShares()
    } else {
      MessagePlugin.error(result.message || t('organization.roleUpdateFailed'))
    }
  } catch (e: unknown) {
    MessagePlugin.error(e instanceof Error ? e.message : t('organization.roleUpdateFailed'))
  }
}

async function handleUnshare(row: ShareRow) {
  try {
    const result = row.kind === 'space'
      ? await orgStore.unshareKnowledgeBase(props.kbId, row.id, row.organizationId || '')
      : await removeUserShare(props.kbId, row.id)
    if (result.success) {
      MessagePlugin.success(t('organization.share.unshareSuccess'))
      await loadShares()
    } else {
      MessagePlugin.error(result.message || t('organization.share.unshareFailed'))
    }
  } catch (e: unknown) {
    MessagePlugin.error(e instanceof Error ? e.message : t('organization.share.unshareFailed'))
  }
}

watch(userQuery, (value) => {
  selectedUserId.value = ''
  userCandidates.value = []
  if (searchTimer) clearTimeout(searchTimer)
  const query = value.trim()
  if (query.length < 2 || shareTargetType.value !== 'user' || !props.kbId) {
    searchingUsers.value = false
    return
  }
  const generation = ++searchGeneration
  searchingUsers.value = true
  searchTimer = setTimeout(async () => {
    try {
      const result = await searchKBUserShareCandidates(props.kbId, query)
      if (generation !== searchGeneration) return
      userCandidates.value = result.success && Array.isArray(result.data) ? result.data : []
    } finally {
      if (generation === searchGeneration) searchingUsers.value = false
    }
  }, 250)
})

watch(shareTargetType, (type) => {
  if (type === 'space') resetUserSearch()
  selectedOrgId.value = ''
})

watch(() => props.kbId, async (newKbId) => {
  if (newKbId) {
    await loadOrganizations()
    await loadShares()
  }
}, { immediate: true })

onBeforeUnmount(() => {
  if (searchTimer) clearTimeout(searchTimer)
})
</script>

<style scoped lang="less">
@import '@/components/share-to-space-panel.less';

.share-add-popup-wide {
  min-width: 360px;
}

.share-target-switch {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px;
  margin: 12px 0 16px;
  padding: 4px;
  background: var(--td-bg-color-secondarycontainer);
  border-radius: var(--app-radius-md);

  button {
    border: 0;
    border-radius: var(--app-radius-sm);
    padding: 8px 12px;
    background: transparent;
    color: var(--td-text-color-secondary);
    cursor: pointer;

    &.active {
      background: var(--td-bg-color-container);
      color: var(--td-brand-color);
      font-weight: 600;
      box-shadow: var(--td-shadow-1);
    }
  }
}

.user-share-search-status {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 4px;
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-sm);
}

.user-share-candidates {
  max-height: 180px;
  overflow: auto;
  margin-top: 6px;
  border: 1px solid var(--td-component-border);
  border-radius: var(--app-radius-sm);
}

.user-share-candidate {
  width: 100%;
  border: 0;
  border-bottom: 1px solid var(--td-component-stroke);
  background: transparent;
  padding: 9px 10px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  text-align: left;
  cursor: pointer;

  &:last-child {
    border-bottom: 0;
  }

  &:hover,
  &.selected {
    background: var(--td-brand-color-light);
  }
}

.user-share-candidate-main {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 2px;

  strong,
  small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  small {
    color: var(--td-text-color-secondary);
  }
}

.user-share-avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--td-brand-color-light);
  color: var(--td-brand-color);
  font-weight: 600;
  flex: 0 0 auto;
}
</style>

<style lang="less">
@import '@/components/share-to-space-panel.overlay.less';
</style>

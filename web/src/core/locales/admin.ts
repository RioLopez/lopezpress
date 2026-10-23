// Admin console (taxonomy, users, backup, review, ban vault, dashboard).

export type AdminTexts = {
  admin: {
    categoriesTitle: string
    categoriesSub: string
    tagsTitle: string
    tagsSub: string
    newCategory: string
    categoryPlaceholder: string
    add: string
    rename: string
    // Spec 25: taxonomies get a local filter box + clickable counts.
    filterPlaceholder: string
    noMatch: string
    viewPosts: string
    // Spec 27: two-level categories + preset tree
    parentCategory: string
    topLevel: string
    applyPreset: string
    presetApplied: string
    presetHint: string
    // Spec 26: admin post management + batch actions
    // NOTE: `status` / `selectAll` already exist in this section (user
    // management) and are reused; only the post-specific strings live here.
    postsTitle: string
    importMd: string
    importDone: string
    postsSub: string
    time: string
    selected: string
    clearSelection: string
    batchPublish: string
    batchPrivate: string
    batchDelete: string
    batchApply: string
    batchConfirm: string
    batchDonePosts: string
    batchSkipped: string
    tagsPlaceholder: string
    name: string
    postCount: string
    deleteConfirmCat: string
    deleteConfirmTag: string
    tagPlaceholder: string
    saved: string
    failed: string
    deleted: string
    emptyCat: string
    emptyTag: string
    notAdmin: string
    // post status badges (spec 22 + 24): rendered as `admin.statusLabel.<value>`
    statusLabel: {
      published: string
      private: string
      pending: string
      rejected: string
      banned: string
      draft: string
    }
    // user management
    usersTitle: string
    usersSub: string
    username: string
    role: string
    status: string
    active: string
    disabled: string
    enable: string
    disable: string
    confirmDisable: string
    confirmEnable: string
    emptyUsers: string
    selectAll: string
    selectedCount: string
    batchEnable: string
    batchDisable: string
    batchConfirmEnable: string
    batchConfirmDisable: string
    batchDone: string
    batchFailed: string
    // backup
    backupTitle: string
    backupSub: string
    exportBtn: string
    exportBusy: string
    exportSuccess: string
    exportFailed: string
    lastBackup: string
    backupStats: string
    importTitle: string
    importSub: string
    importRootOnly: string
    snapshotsTitle: string
    snapshotsSub: string
    snapshotsEmpty: string
    snapshotsName: string
    snapshotsSize: string
    snapshotsTime: string
    restoreBtn: string
    snapshotRestoreConfirm: string
    restoreSuccess: string
    restoreFailed: string
    importBtn: string
    importBusy: string
    backupSecurityNote: string
    importSuccess: string
    importFailed: string
    importConfirm: string
    importNoFile: string
    stUsers: string
    stPosts: string
    stCats: string
    stTags: string
    // trash
    trashTitle: string
    trashSub: string
    restore: string
    restored: string
    restoreConfirm: string
    emptyTrash: string
    emptyTrashBtn: string
    emptyTrashConfirm: string
    emptyTrashDone: string
    deletedAt: string
    expiredHint: string
    // round 19 user management additions
    createUser: string
    newUser: string
    newUsername: string
    newEmail: string
    newPassword: string
    newRole: string
    roleAdmin: string
    roleUser: string
    resetPwd: string
    resetPwdTitle: string
    resetPwdConfirm: string
    newPwdPlaceholder: string
    loginHistory: string
    deleteConfirmUser: string
    deleteUser: string
    lastLoginIp: string
    lastLoginAt: string
    noLogs: string
    loginTime: string
    loginIp: string
    loginOk: string
    loginFail: string
    dashboardTitle: string
    dashboardSub: string
    statPosts: string
    statViews: string
    statPending: string
    trendTitle: string
    trendHint: string
    trendEmpty: string
    topPosts: string
    topEmpty: string
    autoBackupTitle: string
    autoBackupOn: string
    autoBackupOff: string
    autoBackupLast: string
    autoBackupNever: string
    autoBackupRun: string
    autoBackupRunDone: string
    recentPosts: string
    recentLogins: string
    // round 24: three-tier roles + review / ban vault
    roleRoot: string
    roleLocked: string
    rootOnly: string
    reviewTitle: string
    reviewSub: string
    reviewEmpty: string
    submittedAt: string
    approve: string
    reject: string
    rejectTitle: string
    reviewNote: string
    reviewNotePlaceholder: string
    reviewNoteRequired: string
    reviewApproved: string
    reviewRejected: string
    rejectHint: string
    banVaultTitle: string
    banVaultSub: string
    banVaultEmpty: string
    bannedAt: string
    sendBack: string
    sendBackTitle: string
    sendBackHint: string
    sendBackDone: string
    // Spec 30: ban a post from the admin list (root only)
    banPost: string
    banPostConfirm: string
    deleteBanned: string
    deleteBannedConfirm: string
  }
}

export const adminZh: AdminTexts = {
  admin: {
    categoriesTitle: '分类管理',
    categoriesSub: '创建、编辑和删除文章分类',
    tagsTitle: '标签管理',
    tagsSub: '查看和删除已使用的标签',
    newCategory: '新建分类',
    categoryPlaceholder: '分类名称…',
    add: '添加',
    rename: '重命名',
    filterPlaceholder: '按名称筛选…',
    noMatch: '没有匹配项',
    viewPosts: '查看该分类/标签下的文章',
    parentCategory: '父级分类',
    topLevel: '（顶级分类）',
    applyPreset: '应用预设分类',
    presetApplied: '已创建 {created} 个、归位 {reparented} 个分类',
    presetHint: '预设为「技术笔记 / 系统与网络 / 生活随笔」三层结构；同名分类只归位、不重建',
    postsTitle: '文章管理',
    importMd: '导入 Markdown',
    importDone: '导入完成：新建 {created} 篇，跳过 {skipped} 个',
    postsSub: '筛选、编辑、批量处理全站文章',
    time: '发布时间',
    selected: '已选 {n} 篇',
    clearSelection: '取消选择',
    batchPublish: '发布',
    batchPrivate: '转为私密',
    batchDelete: '删除',
    batchApply: '应用',
    batchConfirm: '将对 {n} 篇文章执行「{action}」，确定吗？',
    batchDonePosts: '已处理 {n} 篇',
    batchSkipped: '{n} 篇被跳过（无权限或不存在）',
    tagsPlaceholder: '标签，逗号分隔',
    name: '名称',
    postCount: '文章数',
    deleteConfirmCat: '确定删除分类「{t}」吗？该分类下的文章将变为无分类状态。',
    deleteConfirmTag: '确定删除标签「{t}」吗？相关标签关联将被移除。',
    tagPlaceholder: '',
    saved: '已保存',
    failed: '操作失败',
    deleted: '已删除',
    emptyCat: '暂无分类，在上方添加第一个分类。',
    emptyTag: '暂无标签。',
    notAdmin: '此页面仅限管理员访问。',
    statusLabel: {
      published: '已发布',
      private: '仅自己可见',
      pending: '待审核',
      rejected: '已打回',
      banned: '已封禁',
      draft: '草稿',
    },
    usersTitle: '用户管理',
    usersSub: '查看并启用/禁用所有账号',
    username: '用户名',
    role: '角色',
    status: '状态',
    active: '正常',
    disabled: '已禁用',
    enable: '启用',
    disable: '禁用',
    confirmDisable: '确定禁用用户「{t}」吗？禁用后该账号无法登录。',
    confirmEnable: '确定启用用户「{t}」吗？',
    emptyUsers: '暂无用户。',
    selectAll: '全选',
    selectedCount: '已选 {n} 个用户',
    batchEnable: '批量启用',
    batchDisable: '批量禁用',
    batchConfirmEnable: '确定启用所选的 {n} 个用户吗？',
    batchConfirmDisable: '确定禁用所选的 {n} 个用户吗？禁用后这些账号无法登录。',
    batchDone: '已处理 {n} 个用户',
    batchFailed: '{ok} 个成功，{fail} 个失败',
    backupTitle: '数据备份',
    backupSub: '导出全部数据为 JSON 文件（含已删除文章）',
    exportBtn: '导出备份',
    exportBusy: '正在导出…',
    exportSuccess: '备份已导出',
    exportFailed: '备份导出失败',
    lastBackup: '最近一次备份',
    backupStats: '包含',
    importTitle: '导入备份',
    importSub: '从导出的 JSON 备份文件恢复数据库（将覆盖当前全部数据）',
    importRootOnly: '导入会重建全部数据（含账号与密码哈希），仅 root 可执行',
    snapshotsTitle: '快照恢复',
    snapshotsSub: '导入或恢复前自动创建的数据库快照（保留最近 3 份）。从快照恢复会整体替换当前数据库，恢复前会自动再备份一份，可回退。',
    snapshotsEmpty: '暂无快照（导入或恢复操作会自动创建）',
    snapshotsName: '快照文件',
    snapshotsSize: '大小',
    snapshotsTime: '创建时间',
    restoreBtn: '恢复',
    snapshotRestoreConfirm: '确定用快照 {file} 覆盖当前数据库？当前状态会先自动备份为一份新快照。',
    restoreSuccess: '已从快照恢复',
    restoreFailed: '快照恢复失败',
    importBtn: '选择备份文件',
    importBusy: '正在导入…',
    backupSecurityNote: '⚠️ 安全提醒：备份文件包含 bcrypt 密码哈希，等同于明文口令的离线破解目标。请像密钥一样妥善保管，切勿提交到代码仓库或公开分享。',
    importSuccess: '备份已导入',
    importFailed: '备份导入失败',
    importConfirm: '导入将覆盖当前全部数据且不可撤销，确定继续？',
    importNoFile: '请先选择备份文件',
    stUsers: '位用户',
    stPosts: '文章',
    stCats: '分类',
    stTags: '标签',
    trashTitle: '回收站',
    trashSub: '最近 7 天内删除的文章，可一键恢复',
    restore: '恢复',
    restored: '已恢复',
    restoreConfirm: '确定要恢复「{title}」吗？',
    emptyTrash: '回收站是空的。',
    emptyTrashBtn: '清空回收站',
    emptyTrashConfirm: '确定清空回收站吗？回收站中的文章将被永久删除，无法恢复。',
    emptyTrashDone: '已清空 {n} 篇',
    deletedAt: '删除于',
    expiredHint: '超过 7 天的文章将自动清除，无法恢复。',
    createUser: '新建用户',
    newUser: '新建用户',
    newUsername: '用户名',
    newEmail: '邮箱',
    newPassword: '密码',
    newRole: '角色',
    roleAdmin: '管理员',
    roleUser: '普通用户',
    resetPwd: '重置密码',
    resetPwdTitle: '重置密码',
    resetPwdConfirm: '确定将「{t}」的密码重置为新密码？',
    newPwdPlaceholder: '输入新密码（至少 8 位）',
    loginHistory: '登录历史',
    deleteConfirmUser: '确定删除用户「{t}」吗？该操作不可撤销。',
    deleteUser: '删除',
    lastLoginIp: '最后登录 IP',
    lastLoginAt: '最后登录时间',
    noLogs: '暂无登录记录',
    loginTime: '时间',
    loginIp: 'IP',
    loginOk: '成功',
    loginFail: '失败',
    dashboardTitle: '仪表盘',
    dashboardSub: '站点概览与管理入口',
    statPosts: '文章总数',
    statViews: '总浏览量',
    statPending: '待审核',
    trendTitle: '近 30 天浏览量',
    trendHint: '同一访客同一天只看同一篇计一次，已扣除作者本人与爬虫',
    trendEmpty: '近 30 天还没有阅读记录',
    topPosts: '最多阅读',
    topEmpty: '还没有读过任何文章',
    autoBackupTitle: '自动备份',
    autoBackupOn: '已开启：每天自动快照一次',
    autoBackupOff: '未开启',
    autoBackupLast: '上次快照',
    autoBackupNever: '还没有快照',
    autoBackupRun: '立即快照',
    autoBackupRunDone: '已生成快照：{file}',
    recentPosts: '最近文章',
    recentLogins: '最近登录',
    roleRoot: '超级管理员',
    roleLocked: '该账号的角色不允许修改',
    rootOnly: '此页面仅限超级管理员访问。',
    reviewTitle: '内容审核',
    reviewSub: '普通用户的文章由管理员审核，管理员的文章由超级管理员审核',
    reviewEmpty: '暂无待审核文章',
    submittedAt: '提交时间',
    approve: '通过',
    reject: '打回',
    rejectTitle: '打回修改',
    reviewNote: '审核意见',
    reviewNotePlaceholder: '说明需要修改的地方（作者可见）',
    reviewNoteRequired: '打回时必须填写审核意见',
    reviewApproved: '已通过审核',
    reviewRejected: '已打回给作者修改',
    rejectHint: '作者会收到打回通知和上面的意见，可修改后重新提交。',
    banVaultTitle: '封禁库',
    banVaultSub: '被超级管理员封禁的文章，仅超级管理员可见',
    banVaultEmpty: '封禁库为空',
    bannedAt: '封禁时间',
    sendBack: '发回原作者',
    sendBackTitle: '发回原作者修改',
    sendBackHint: '文章会从封禁库移出，作者可修改后重新提交。',
    sendBackDone: '已发回原作者',
    banPost: '封禁',
    banPostConfirm: '封禁后这篇文章只有你能看到，确定吗？',
    deleteBanned: '永久删除',
    deleteBannedConfirm: '确定永久删除「{t}」吗？该操作不可撤销。',
  }
}

export const adminEn: AdminTexts = {
  admin: {
    categoriesTitle: 'Manage Categories',
    categoriesSub: 'Create, edit and delete post categories',
    tagsTitle: 'Manage Tags',
    tagsSub: 'View and delete in-use tags',
    newCategory: 'New category',
    categoryPlaceholder: 'Category name…',
    add: 'Add',
    rename: 'Rename',
    filterPlaceholder: 'Filter by name…',
    noMatch: 'No matches',
    viewPosts: 'View the posts in this category/tag',
    parentCategory: 'Parent category',
    topLevel: '(top level)',
    applyPreset: 'Apply preset categories',
    presetApplied: 'Created {created}, re-parented {reparented}',
    presetHint: 'Preset: 技术笔记 / 系统与网络 / 生活随笔. Existing names are adopted, never recreated',
    postsTitle: 'Posts',
    importMd: 'Import Markdown',
    importDone: 'Imported {created} post(s), skipped {skipped}',
    postsSub: 'Filter, edit and bulk-manage every post',
    time: 'Published',
    selected: '{n} selected',
    clearSelection: 'Clear selection',
    batchPublish: 'Publish',
    batchPrivate: 'Make private',
    batchDelete: 'Delete',
    batchApply: 'Apply',
    batchConfirm: 'Apply "{action}" to {n} post(s)?',
    batchDonePosts: '{n} post(s) updated',
    batchSkipped: '{n} skipped (no permission or missing)',
    tagsPlaceholder: 'Tags, comma separated',
    name: 'Name',
    postCount: 'Posts',
    deleteConfirmCat: 'Delete category "{t}"? Posts in this category will become uncategorized.',
    deleteConfirmTag: 'Delete tag "{t}"? Tag associations will be removed.',
    tagPlaceholder: '',
    saved: 'Saved',
    failed: 'Operation failed',
    deleted: 'Deleted',
    emptyCat: 'No categories yet. Add your first one above.',
    emptyTag: 'No tags yet.',
    notAdmin: 'This page is admin-only.',
    statusLabel: {
      published: 'Published',
      private: 'Private',
      pending: 'Pending review',
      rejected: 'Sent back',
      banned: 'Banned',
      draft: 'Draft',
    },
    usersTitle: 'Manage Users',
    usersSub: 'View and enable/disable all accounts',
    username: 'Username',
    role: 'Role',
    status: 'Status',
    active: 'Active',
    disabled: 'Disabled',
    enable: 'Enable',
    disable: 'Disable',
    confirmDisable: 'Disable user "{t}"? They will not be able to sign in.',
    confirmEnable: 'Enable user "{t}"?',
    emptyUsers: 'No users.',
    selectAll: 'Select all',
    selectedCount: '{n} user(s) selected',
    batchEnable: 'Enable selected',
    batchDisable: 'Disable selected',
    batchConfirmEnable: 'Enable the {n} selected user(s)?',
    batchConfirmDisable: 'Disable the {n} selected user(s)? They will no longer be able to sign in.',
    batchDone: 'Processed {n} user(s)',
    batchFailed: '{ok} succeeded, {fail} failed',
    backupTitle: 'Data Backup',
    backupSub: 'Export all data to a JSON file (includes deleted posts)',
    exportBtn: 'Export backup',
    exportBusy: 'Exporting…',
    exportSuccess: 'Backup exported',
    exportFailed: 'Failed to export backup',
    lastBackup: 'Last backup',
    backupStats: 'Contains',
    importTitle: 'Import backup',
    importSub: 'Restore the database from an exported JSON file (replaces all current data)',
    importRootOnly: 'Import rebuilds all data (accounts and password hashes included); root only',
    snapshotsTitle: 'Snapshot restore',
    snapshotsSub: 'Database snapshots taken automatically before an import or restore (3 most recent kept). Restoring replaces the whole database, auto-backing-up the current state first so the restore is undoable.',
    snapshotsEmpty: 'No snapshots yet (imports and restores create them automatically)',
    snapshotsName: 'Snapshot file',
    snapshotsSize: 'Size',
    snapshotsTime: 'Created',
    restoreBtn: 'Restore',
    snapshotRestoreConfirm: 'Overwrite the current database with snapshot {file}? The current state is auto-backed-up as a new snapshot first.',
    restoreSuccess: 'Restored from snapshot',
    restoreFailed: 'Snapshot restore failed',
    importBtn: 'Choose backup file',
    importBusy: 'Importing…',
    backupSecurityNote: '⚠️ Security note: the backup embeds bcrypt password hashes — an offline-cracking target equivalent to the raw passwords. Store it like a secret: restrict access, never commit to version control or share publicly.',
    importSuccess: 'Backup imported',
    importFailed: 'Failed to import backup',
    importConfirm: 'Importing will replace all current data and cannot be undone. Continue?',
    importNoFile: 'Choose a backup file first',
    stUsers: ' users',
    stPosts: 'posts',
    stCats: 'categories',
    stTags: 'tags',
    trashTitle: 'Trash',
    trashSub: 'Posts deleted in the last 7 days — restore in one click',
    restore: 'Restore',
    restored: 'Restored',
    restoreConfirm: 'Restore “{title}”?',
    emptyTrash: 'Trash is empty.',
    emptyTrashBtn: 'Empty trash',
    emptyTrashConfirm: 'Empty the trash? Posts in the trash will be permanently deleted and cannot be recovered.',
    emptyTrashDone: 'Deleted {n} post(s)',
    deletedAt: 'Deleted',
    expiredHint: 'Posts older than 7 days are cleared automatically and cannot be restored.',
    createUser: 'New user',
    newUser: 'New user',
    newUsername: 'Username',
    newEmail: 'Email',
    newPassword: 'Password',
    newRole: 'Role',
    roleAdmin: 'Admin',
    roleUser: 'User',
    resetPwd: 'Reset password',
    resetPwdTitle: 'Reset password',
    resetPwdConfirm: 'Reset the password for "{t}" to the new value?',
    newPwdPlaceholder: 'New password (at least 8 chars)',
    loginHistory: 'Login history',
    deleteConfirmUser: 'Delete user "{t}"? This cannot be undone.',
    deleteUser: 'Delete',
    lastLoginIp: 'Last login IP',
    lastLoginAt: 'Last login time',
    noLogs: 'No login records',
    loginTime: 'Time',
    loginIp: 'IP',
    loginOk: 'Success',
    loginFail: 'Failed',
    dashboardTitle: 'Dashboard',
    dashboardSub: 'Site overview and management',
    statPosts: 'Total posts',
    statViews: 'Total views',
    statPending: 'Awaiting review',
    trendTitle: 'Views, last 30 days',
    trendHint: 'One read per visitor per post per day; the author and crawlers are excluded',
    trendEmpty: 'No reads in the last 30 days yet',
    topPosts: 'Most read',
    topEmpty: 'No post has been read yet',
    autoBackupTitle: 'Automatic backup',
    autoBackupOn: 'On: one snapshot per day',
    autoBackupOff: 'Off',
    autoBackupLast: 'Last snapshot',
    autoBackupNever: 'No snapshot yet',
    autoBackupRun: 'Snapshot now',
    autoBackupRunDone: 'Snapshot written: {file}',
    recentPosts: 'Recent posts',
    recentLogins: 'Recent logins',
    roleRoot: 'Super admin',
    roleLocked: 'This account\'s role cannot be changed',
    rootOnly: 'This page is restricted to the super admin.',
    reviewTitle: 'Content review',
    reviewSub: 'Posts by users are reviewed by admins; posts by admins are reviewed by the super admin',
    reviewEmpty: 'No posts awaiting review',
    submittedAt: 'Submitted',
    approve: 'Approve',
    reject: 'Reject',
    rejectTitle: 'Send back for revision',
    reviewNote: 'Review note',
    reviewNotePlaceholder: 'Explain what needs fixing (visible to the author)',
    reviewNoteRequired: 'A review note is required when rejecting',
    reviewApproved: 'Post approved',
    reviewRejected: 'Sent back to the author',
    rejectHint: 'The author is notified with your note and can revise and resubmit.',
    banVaultTitle: 'Ban vault',
    banVaultSub: 'Banned posts — visible to the super admin only',
    banVaultEmpty: 'The ban vault is empty',
    bannedAt: 'Banned',
    sendBack: 'Send back to author',
    sendBackTitle: 'Send back to author',
    sendBackHint: 'The post leaves the vault and the author can revise and resubmit.',
    sendBackDone: 'Sent back to the author',
    banPost: 'Ban',
    banPostConfirm: 'A banned post is visible to you only. Continue?',
    deleteBanned: 'Delete permanently',
    deleteBannedConfirm: 'Permanently delete "{t}"? This cannot be undone.',
  }
}

// Site settings, audit trail and image library.

export type SiteTexts = {
  site: {
    title: string
    basic: string
    siteName: string
    siteTitle: string
    logo: string
    favicon: string
    footer: string
    icp: string
    friendLinks: string
    friendName: string
    friendUrl: string
    addLink: string
    imgHost: string
    mode: string
    local: string
    smms: string
    token: string
    tokenHint: string
    // Spec 24: public list pagination
    display: string
    pageSize: string
    pageSizeHint: string
    publicRead: string
    publicReadHint: string
    autoBackup: string
    autoBackupHint: string
    backupKeep: string
    backupKeepHint: string
    register: string
    registerOpen: string
    registerOpenHint: string
    rootOnlyHint: string
    // Spec 36: image storage policy (three ceilings + two author switches).
    storage: string
    storageHint: string
    uploadMaxImage: string
    uploadMaxAvatar: string
    userQuota: string
    siteQuota: string
    allowUserDeleteUpload: string
    allowUserChangeAvatar: string
    allowExternalAvatar: string
    externalAvatarHint: string
    // Spec 37 (experimental email verification)
    emailVerify: string
    emailVerifyHint: string
    emailVerifyEnable: string
    codeTtl: string
    codeCooldown: string
    // Three-tier role / permission matrix (read-only reference)
    permissions: string
    permissionsHint: string
    permColRole: string
    permRowManageUsers: string
    permRowManageContent: string
    permRowReview: string
    permRowBan: string
    permRowBackup: string
    permRowSiteConfig: string
    permRowAuthorPost: string
    permRowSelfEdit: string
    permRoot: string
    permAdmin: string
    permUser: string
    permYes: string
    permNo: string
    permOwnOnly: string
    // security notifications (SMTP)
    mail: string
    mailHost: string
    mailPort: string
    mailUser: string
    mailPass: string
    mailEnabled: string
    mailTest: string
    mailTestSent: string
    mailTestFailed: string
    mailDisabledHint: string
    save: string
    saved: string
    saveFailed: string
    nameTooLong: string
    siteUrl: string
    footerRss: string
    footerSitemap: string
  },
  audit: {
    title: string
    sub: string
    empty: string
    limit: string
    time: string
    actor: string
    action: string
    target: string
    detail: string
    verb: {
      create_post: string
      update_post: string
      delete_post: string
      restore_post: string
      empty_trash: string
      create_user: string
      update_user: string
      set_user_disabled: string
      reset_password: string
      delete_user: string
      update_site_config: string
      export_backup: string
      import_backup: string
    }
  },
  // "My uploads" — the author-side view. It is deliberately NOT called a
  // library: a plain user only ever sees their own files (spec 36 §1).
  myUploads: {
    title: string
    sub: string
    empty: string
    delete: string
    confirm: string
    deleted: string
    deleteFailed: string
    loading: string
    used: string
    noQuota: string
    refNote: string
  }
}

export const siteZh: SiteTexts = {
  site: {
    title: '站点设置',
    basic: '基本信息',
    siteName: '站点名称',
    siteTitle: '浏览器标题',
    logo: '站点 Logo',
    favicon: 'Favicon',
    footer: '页脚版权文本',
    icp: '备案号',
    friendLinks: '友情链接',
    friendName: '名称',
    friendUrl: '链接',
    addLink: '添加链接',
    imgHost: '图床',
    mode: '模式',
    local: '本地',
    smms: 'SM.MS',
    token: 'SM.MS Token',
    tokenHint: '服务端保存，浏览器不再可见',
    display: '显示设置',
    pageSize: '首页每页文章数',
    pageSizeHint: '读者不可自行修改，保证分页链接对所有人一致',
    publicRead: '公开阅读',
    publicReadHint: '开启后未登录访客也能浏览文章；关闭则必须登录才能看',
    autoBackup: '每日自动备份',
    autoBackupHint: '开启后每天生成一份数据库快照，出错可一键回到某一天；占用等于数据库文件大小',
    backupKeep: '快照保留份数',
    backupKeepHint: '超过份数的最旧快照会被自动删除（1–90 天）',
    register: '公开注册',
    registerOpen: '允许访客自行注册账号',
    registerOpenHint: '关闭后访客无法注册，只能由管理员在后台创建账号',
    rootOnlyHint: '仅 root 可修改（当前值仍然生效）',
    storage: '图片与存储',
    storageHint: '三层上限，从窄到宽：单文件、单个账号、整个站点。单位 MB。',
    uploadMaxImage: '正文图片单文件上限',
    uploadMaxAvatar: '头像单文件上限',
    userQuota: '每个账号的总配额',
    siteQuota: '站点磁盘总配额',
    allowUserDeleteUpload: '允许作者撤下自己上传的图片',
    allowUserChangeAvatar: '允许用户自行更换头像',
    allowExternalAvatar: '允许使用站外图片作为头像',
    externalAvatarHint: '关闭时头像只能从本站图库选择，避免被人塞入追踪像素。',
    emailVerify: '邮箱验证',
    emailVerifyHint: '开启后，注册、找回密码、换绑邮箱都需要邮箱验证码。需先配置好 SMTP，否则验证码发不出去。',
    emailVerifyEnable: '注册与找回密码需要邮箱验证码',
    codeTtl: '验证码有效期（分钟，2–60）',
    codeCooldown: '同一邮箱重发间隔（秒，30–600）',
    permissions: '角色权限说明',
    permissionsHint: '三级角色（超级管理员 / 管理员 / 普通用户）的权限对照，仅供查看，规则由系统固化。',
    permColRole: '权限',
    permRowManageUsers: '管理用户',
    permRowManageContent: '管理分类 / 标签',
    permRowReview: '审核文章',
    permRowBan: '封禁 / 解封文章',
    permRowBackup: '备份导入 / 快照恢复',
    permRowSiteConfig: '站点设置',
    permRowAuthorPost: '发表文章',
    permRowSelfEdit: '编辑 / 删除自己的文章',
    permRoot: '超级管理员',
    permAdmin: '管理员',
    permUser: '普通用户',
    permYes: '可',
    permNo: '不可',
    permOwnOnly: '仅自己',
    mail: '安全通知（SMTP 邮件）',
    mailHost: 'SMTP 主机',
    mailPort: 'SMTP 端口（587 或 465）',
    mailUser: '发件账号',
    mailPass: '邮箱密码 / 授权码',
    mailEnabled: '主机留空则关闭邮件通知',
    mailTest: '发送测试邮件',
    mailTestSent: '测试邮件已发送',
    mailTestFailed: '测试邮件发送失败',
    mailDisabledHint: '开启后，登录失败、修改密码、异地登录将向账号邮箱发送安全提醒（最多每 15 分钟一封登录失败提醒）。',
    save: '保存设置',
    saved: '设置已保存',
    saveFailed: '设置保存失败',
    nameTooLong: '名称过长',
    siteUrl: '站点地址（用于 RSS / 站点地图的绝对链接，留空则自动推断）',
    footerRss: 'RSS 订阅',
    footerSitemap: '站点地图',
  },
  audit: {
    title: '操作日志',
    sub: '站内内容与管理操作的审计记录（谁在何时做了什么）',
    empty: '暂无操作记录',
    limit: '显示条数',
    time: '时间',
    actor: '操作人',
    action: '操作',
    target: '对象',
    detail: '详情',
    verb: {
      create_post: '新建文章',
      update_post: '更新文章',
      delete_post: '删除文章',
      restore_post: '恢复文章',
      empty_trash: '清空回收站',
      create_user: '新建用户',
      update_user: '修改用户',
      set_user_disabled: '禁用/启用用户',
      reset_password: '重置密码',
      delete_user: '删除用户',
      update_site_config: '修改站点设置',
      export_backup: '导出备份',
      import_backup: '导入备份',
    },
  },
  myUploads: {
    title: '我的图片',
    sub: '只有你能看到这个列表。删除只是撤下：文件仍会保留给已经引用它的文章，但其他人再也拿不到它。',
    empty: '你还没有上传过图片',
    delete: '撤下',
    confirm: '撤下这张图片？图库中不会再出现它，别人也无法再使用它。',
    deleted: '图片已撤下',
    deleteFailed: '操作失败',
    loading: '加载中…',
    used: '已用 {used} / {quota}',
    noQuota: '站点未设置个人配额',
    refNote: '被 {n} 篇文章引用',
  }
}

export const siteEn: SiteTexts = {
  site: {
    title: 'Site Settings',
    basic: 'Basic info',
    siteName: 'Site name',
    siteTitle: 'Browser title',
    logo: 'Logo',
    favicon: 'Favicon',
    footer: 'Footer copyright text',
    icp: 'ICP / license no.',
    friendLinks: 'Friend links',
    friendName: 'Name',
    friendUrl: 'URL',
    addLink: 'Add link',
    imgHost: 'Image hosting',
    mode: 'Mode',
    local: 'Local',
    smms: 'SM.MS',
    token: 'SM.MS Token',
    tokenHint: 'Stored on the server; never exposed to the browser',
    display: 'Display',
    pageSize: 'Posts per page',
    pageSizeHint: 'Readers cannot change this, so a pagination link means the same thing for everyone',
    publicRead: 'Public reading',
    publicReadHint: 'When on, visitors can read without signing in; when off, reading requires a login',
    autoBackup: 'Daily automatic backup',
    autoBackupHint: 'One database snapshot per day so any day can be restored; costs disk equal to the database size',
    backupKeep: 'Snapshots to keep',
    backupKeepHint: 'Older snapshots beyond this count are deleted automatically (1–90 days)',
    register: 'Public registration',
    registerOpen: 'Allow visitors to self-register',
    registerOpenHint: 'When off, visitors cannot register; accounts must be created by an admin',
    rootOnlyHint: 'Root only (the current value stays in effect)',
    storage: 'Images & storage',
    storageHint: 'Three ceilings, narrow to wide: one file, one account, the whole site. In MB.',
    uploadMaxImage: 'Max size per article image',
    uploadMaxAvatar: 'Max size per avatar',
    userQuota: 'Quota per account',
    siteQuota: 'Total disk quota for the site',
    allowUserDeleteUpload: 'Authors may withdraw their own images',
    allowUserChangeAvatar: 'Users may change their own avatar',
    allowExternalAvatar: 'Allow avatars hosted elsewhere',
    externalAvatarHint: 'When off, avatars must come from this site’s library — an external URL is a tracking pixel with a friendly name.',
    emailVerify: 'Email verification',
    emailVerifyHint: 'When on, sign-up, password reset and email changes require a code. SMTP must be configured first, otherwise no code can be delivered.',
    emailVerifyEnable: 'Require an email code to register and to reset a password',
    codeTtl: 'Code lifetime (minutes, 2–60)',
    codeCooldown: 'Resend interval per address (seconds, 30–600)',
    permissions: 'Role permissions',
    permissionsHint: 'Reference matrix for the three tiers (super admin / admin / user). Read-only; the rules are fixed in code.',
    permColRole: 'Permission',
    permRowManageUsers: 'Manage users',
    permRowManageContent: 'Manage categories / tags',
    permRowReview: 'Review posts',
    permRowBan: 'Ban / unban posts',
    permRowBackup: 'Backup import / snapshot restore',
    permRowSiteConfig: 'Site settings',
    permRowAuthorPost: 'Author posts',
    permRowSelfEdit: 'Edit / delete own posts',
    permRoot: 'Super admin',
    permAdmin: 'Admin',
    permUser: 'User',
    permYes: 'Yes',
    permNo: 'No',
    permOwnOnly: 'Own only',
    mail: 'Security notifications (SMTP)',
    mailHost: 'SMTP host',
    mailPort: 'SMTP port (587 or 465)',
    mailUser: 'Sender account',
    mailPass: 'Mail password / app password',
    mailEnabled: 'Leave host empty to disable notifications',
    mailTest: 'Send test email',
    mailTestSent: 'Test email sent',
    mailTestFailed: 'Failed to send test email',
    mailDisabledHint: 'When enabled, login failures, password changes and new-location logins email a security alert to the account inbox (login-failure alerts are throttled to one per 15 minutes).',
    save: 'Save settings',
    saved: 'Settings saved',
    saveFailed: 'Failed to save settings',
    nameTooLong: 'Name too long',
    siteUrl: 'Site URL (absolute links for RSS / sitemap; auto-detected if blank)',
    footerRss: 'RSS feed',
    footerSitemap: 'Sitemap',
  },
  audit: {
    title: 'Operation log',
    sub: 'Audit trail of content & admin actions on the site',
    empty: 'No recorded operations yet',
    limit: 'Show',
    time: 'Time',
    actor: 'Actor',
    action: 'Action',
    target: 'Target',
    detail: 'Detail',
    verb: {
      create_post: 'Create post',
      update_post: 'Update post',
      delete_post: 'Delete post',
      restore_post: 'Restore post',
      empty_trash: 'Empty trash',
      create_user: 'Create user',
      update_user: 'Update user',
      set_user_disabled: 'Enable/disable user',
      reset_password: 'Reset password',
      delete_user: 'Delete user',
      update_site_config: 'Change site settings',
      export_backup: 'Export backup',
      import_backup: 'Import backup',
    },
  },
  myUploads: {
    title: 'My images',
    sub: 'Only you can see this list. Deleting withdraws an image: the file stays for articles that already use it, but nobody else can obtain it.',
    empty: "You haven't uploaded any images yet",
    delete: 'Withdraw',
    confirm: 'Withdraw this image? It disappears from the library and nobody else can use it.',
    deleted: 'Image withdrawn',
    deleteFailed: 'Action failed',
    loading: 'Loading…',
    used: '{used} of {quota} used',
    noQuota: 'No per-user quota is configured',
    refNote: 'Used by {n} article(s)',
  }
}

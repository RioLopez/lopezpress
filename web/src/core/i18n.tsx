import { createContext } from 'preact'
import { useContext, useEffect, useState } from 'preact/hooks'
import type { ComponentChildren } from 'preact'

export type Locale = 'zh' | 'en'

type Texts = {
  nav: {
    list: string
    create: string
    archive: string
    settings: string
    admin: string
    backToContent: string
    // Spec 26: admin sidebar is grouped instead of 10 flat items.
    groupContent: string
    groupUsers: string
    groupSystem: string
    dashboard: string
    site: string
  }
  menu: {
    profile: string
    account: string
    logout: string
    loginOrRegister: string
    confirmLogout: string
  }
  center: {
    title: string
    myHome: string
    contentManage: string
    security: string
    resubmitted: string
    resubmitFailed: string
    bannerViews: string
    bannerPosts: string
    registeredAt: string
    bio: string
    editProfile: string
    postsList: string
    sortBy: string
    sortTime: string
    sortViews: string
    orderAsc: string
    orderDesc: string
    empty: string
    manageEmpty: string
    status: string
    edit: string
    account: string
    username: string
    maskedHint: string
    show: string
    hide: string
    securityEmpty: string
    securityNote: string
    secChangePw: string
    secCurrentPw: string
    secNewPw: string
    secConfirmPw: string
    secPwBtn: string
    secPwChanged: string
    secPwMismatch: string
    secPwTooShort: string
    secPwWrong: string
    secLoginHistory: string
    secNoLogins: string
    secLoginOk: string
    secLoginFail: string
    secIp: string
    secDevice: string
    secLogoutAll: string
    secLogoutAllConfirm: string
    secLogoutAllDone: string
    loading: string
    loadError: string
    retry: string
  }
  auth: {
    loginTitle: string
    loginSub: string
    registerTitle: string
    registerSub: string
    logout: string
    welcomeBack: string
    loginFailed: string
    registerFailed: string
    noAccount: string
    hasAccount: string
    empty: string
    username: string
    usernamePlaceholder: string
    password: string
    passwordPlaceholder: string
    showPwd: string
    hidePwd: string
    email: string
    loginBtn: string
    registering: string
    loginBusy: string
    passwordTooShort: string
  }
  password: {
    title: string
    subtitle: string
    alreadySet: string
    continue: string
    fillAll: string
    mismatch: string
    tooShort: string
    changed: string
    changeFailed: string
    newLabel: string
    confirmLabel: string
    currentPlaceholder: string
    newPlaceholder: string
    busy: string
    submit: string
  }
  list: {
    title: string
    searchPlaceholder: string
    allStatus: string
    published: string
    author: string
    edit: string
    del: string
    empty: string
    emptyLoggedOut: string
    // Spec 28: filtered result differs from "no posts at all".
    noResults: string
    clearFilters: string
    deleteConfirm: string
    prevPage: string
    nextPage: string
    // Spec 24: classic pager labels.
    pagination: string
    firstPage: string
    lastPage: string
    jumpTo: string
    jumpPage: string
    pageOutOfRange: string
    pageOf: string
    allCategories: string
    categoryLabel: string
    tagLabel: string
    pinned: string
    views: string
    loadingMore: string
    noMore: string
    loadMore: string
    // Spec 22: visibility labels for the content-management list. "private"
    // is a first-class badge.
    private: string
    visibility: string
    setPrivate: string
    setPublic: string
    visibilityUpdated: string
    visibilityUndo: string
    visibilityFailed: string
  }
  sidebar: {
    categories: string
    tags: string
    recent: string
    allCategories: string
  }
  post: {
    loading: string
    notFound: string
    backToList: string
    created: string
    updated: string
    deleted: string
    deleteConfirm: string
    loadFailed: string
    emptyBody: string
    words: string
    views: string
    prevPost: string
    nextPost: string
    pinned: string
  }
  editor: {
    title: string
    body: string
    status: string
    category: string
    tags: string
    tagPlaceholder: string
    allCategories: string
    publish: string
    cancel: string
    save: string
    created: string
    saved: string
    createFailed: string
    saveFailed: string
    titlePlaceholder: string
    heading: string
    bodyPlaceholder: string
    preview: string
    editBtn: string
    h1: string
    h2: string
    h3: string
    bold: string
    italic: string
    code: string
    blockquote: string
    link: string
    dragDivider: string
    image: string
    list: string
    autosaveSaved: string
    pinnedLabel: string
    pinnedHint: string
    stats: string
    readTime: string
    visibility: string
    visibilityPublic: string
    visibilityPrivate: string
    visibilityPublicHint: string
    visibilityPrivateHint: string
    visibilityReviewHint: string
    rejectedBanner: string
    resubmit: string
    modeSplit: string
    modeIR: string
    modeSource: string
    cover: string
    coverUpload: string
    coverFromBody: string
    coverClear: string
    restoreDraft: string
    restoreDraftConfirm: string
    restoreDraftDiscard: string
    savedAt: string
    publishing: string
    publishUpdate: string
    toc: string
    metaPanel: string
  }
  imgUpload: {
    uploadLocal: string
    localHint: string
    uploaded: string
    failed: string
    tooLarge: string
    badFormat: string
    noFile: string
  }
  rev: {
    title: string
    empty: string
    restore: string
    confirm: string
    restored: string
    chars: string
  }
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
  }
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
  }
  profile: {
    title: string
    displayName: string
    bio: string
    avatar: string
    avatarHint: string
    save: string
    saved: string
    saveFailed: string
  }
  imageLib: {
    title: string
    sub: string
    upload: string
    empty: string
    delete: string
    confirm: string
    deleted: string
    deleteFailed: string
    loading: string
    size: string
    modified: string
  }
  author: {
    title: string
    posts: string
    none: string
  }
  register: {
    closed: string
    contactAdmin: string
  }
  read: {
    font: string
    fontSmall: string
    fontMedium: string
    fontLarge: string
    fontXlarge: string
    reset: string
    minutes: string
    read: string
    copyCode: string
    codeCopied: string
    tocTitle: string
    related: string
    home: string
    // Spec 29: permanent link
    permanentLink: string
    linkCopied: string
    tocOpen: string
    tocClose: string
    backToTop: string
    lightboxClose: string
  }
  notfound: {
    text: string
    home: string
  }
  archive: {
    title: string
    empty: string
    emptyLoggedOut: string
    loadFailed: string
    total: string
  }
  common: {
    loading: string
    count: string
  }
}

const zh: Texts = {
  nav: { list: '全部文章', create: '写文章', archive: '归档', settings: '设置', admin: '管理', backToContent: '返回内容', dashboard: '仪表盘', site: '站点设置', groupContent: '内容', groupUsers: '用户', groupSystem: '系统' },
  menu: { profile: '个人中心', account: '账号设置', logout: '退出登录', loginOrRegister: '登录 / 注册', confirmLogout: '确定退出登录吗？' },
  center: {
    title: '个人中心',
    myHome: '我的主页',
    contentManage: '内容管理',
    resubmitted: '已重新提交审核',
    resubmitFailed: '重新提交失败',
    security: '安全中心',
    bannerViews: '总访问量',
    bannerPosts: '文章数',
    registeredAt: '注册时间',
    bio: '个人简介',
    editProfile: '编辑资料',
    postsList: '我的文章',
    sortBy: '排序',
    sortTime: '发布时间',
    sortViews: '浏览量',
    orderAsc: '正序',
    orderDesc: '倒序',
    empty: '暂无文章',
    manageEmpty: '暂无内容',
    status: '状态',
    edit: '编辑',
    account: '账号',
    username: '用户名',
    maskedHint: '敏感信息已部分脱敏',
    show: '显示',
    hide: '隐藏',
    securityEmpty: '暂无账号信息',
    securityNote: '安全中心：修改密码、查看登录记录、一键登出所有设备。',
    secChangePw: '修改密码',
    secCurrentPw: '当前密码',
    secNewPw: '新密码',
    secConfirmPw: '确认新密码',
    secPwBtn: '更新密码',
    secPwChanged: '密码已更新',
    secPwMismatch: '两次输入的新密码不一致',
    secPwTooShort: '新密码至少 8 位',
    secPwWrong: '当前密码错误',
    secLoginHistory: '登录记录',
    secNoLogins: '暂无登录记录',
    secLoginOk: '成功',
    secLoginFail: '失败',
    secIp: 'IP',
    secDevice: '设备',
    secLogoutAll: '登出所有设备',
    secLogoutAllConfirm: '确定登出所有设备吗？当前及其他设备的登录会话都会失效。',
    secLogoutAllDone: '已登出所有设备，请重新登录',
    loading: '加载中…',
    loadError: '加载失败',
    retry: '重试',
  },
  auth: {
    loginTitle: '登录',
    loginSub: '使用用户名或邮箱登录',
    registerTitle: '注册',
    registerSub: '创建新账号，注册后自动登录',
    logout: '登出',
    welcomeBack: '欢迎回来，',
    loginFailed: '登录失败',
    registerFailed: '注册失败',
    noAccount: '还没有账号？',
    hasAccount: '已有账号？',
    empty: '请填写完整信息',
    username: '用户名 / 邮箱',
    usernamePlaceholder: '用户名或邮箱',
    password: '密码',
    passwordPlaceholder: '请输入密码',
    showPwd: '显示',
    hidePwd: '隐藏',
    email: '邮箱',
    loginBtn: '登录',
    registering: '注册',
    loginBusy: '登录中…',
    passwordTooShort: '密码至少 8 位',
  },
  password: {
    title: '首次登录，请修改密码',
    subtitle: '系统为你生成了临时密码，请设置自己的密码后继续。',
    alreadySet: '无需改密，直接进入。',
    continue: '继续',
    fillAll: '请填写所有字段',
    mismatch: '两次输入的新密码不一致',
    tooShort: '新密码至少 8 个字符',
    changed: '密码已修改',
    changeFailed: '修改失败，请检查当前密码是否正确',
    newLabel: '新密码',
    confirmLabel: '确认新密码',
    currentPlaceholder: '当前（临时）密码',
    newPlaceholder: '至少 8 个字符',
    busy: '修改中…',
    submit: '修改密码',
  },
  list: {
    title: '文章列表',
    searchPlaceholder: '搜索标题/正文/作者…',
    allStatus: '全部状态',
    published: '已发布',
    author: '作者',
    edit: '编辑',
    del: '删除',
    empty: '暂无文章。点击上方按钮写第一篇。',
    emptyLoggedOut: '请先登录。',
    noResults: '没有找到匹配的文章，换个关键词或分类试试。',
    clearFilters: '清除筛选',
    deleteConfirm: '确定删除这篇文章吗？',
    prevPage: '上一页',
    nextPage: '下一页',
    pagination: '分页',
    firstPage: '首页',
    lastPage: '末页',
    jumpTo: '跳至',
    jumpPage: '页',
    pageOutOfRange: '这一页没有内容，链接可能已经过期。',
    pageOf: '第 {page} / {total} 页',
    allCategories: '全部分类',
    categoryLabel: '分类',
    tagLabel: '标签',
    pinned: '置顶',
    views: '浏览',
    loadingMore: '加载中…',
    noMore: '没有更多了',
    loadMore: '加载更多',
    private: '私密',
    visibility: '可见性',
    setPrivate: '设为私密',
    setPublic: '设为公开',
    visibilityUpdated: '可见性已更新',
    visibilityUndo: '撤销',
    visibilityFailed: '可见性更新失败',
  },
  sidebar: {
    categories: '分类',
    tags: '标签',
    recent: '最近文章',
    allCategories: '全部分类',
  },
  post: {
    loading: '加载中…',
    notFound: '未找到文章',
    backToList: '返回列表',
    created: '创建于',
    updated: '更新于',
    deleted: '已删除',
    deleteConfirm: '确定删除《{t}》吗？',
    loadFailed: '加载失败',
    emptyBody: '（无正文）',
    words: '字',
    views: '次浏览',
    prevPost: '上一篇',
    nextPost: '下一篇',
    pinned: '置顶',
  },
  editor: {
    title: '标题',
    body: '正文（支持 Markdown）',
    status: '状态',
    category: '分类',
    tags: '标签',
    tagPlaceholder: '输入标签，逗号分隔…',
    allCategories: '全部分类',
    publish: '发布',
    cancel: '取消',
    save: '保存修改',
    created: '文章已创建',
    saved: '已保存',
    createFailed: '创建失败',
    saveFailed: '保存失败',
    titlePlaceholder: '文章标题',
    heading: '写文章',
    bodyPlaceholder: '在这里写正文…',
    preview: '预览',
    editBtn: '编辑',
    h1: '标题 1',
    h2: '标题 2',
    h3: '标题 3',
    bold: '粗体',
    italic: '斜体',
    code: '代码',
    blockquote: '引用',
    link: '链接',
    image: '图片',
    list: '列表',
    autosaveSaved: '已自动保存',
    pinnedLabel: '置顶文章',
    pinnedHint: '置顶后文章将始终显示在列表最前面',
    dragDivider: '拖动调整比例',
    stats: '{n} 字',
    readTime: '约 {n} 分钟读完',
    visibility: '可见范围',
    visibilityPublic: '公开',
    visibilityPrivate: '私密',
    visibilityPublicHint: '未登录读者也能浏览这篇文章',
    visibilityPrivateHint: '只有你自己能看到，其他人访问会显示 404',
    visibilityReviewHint: '本文需经审核通过后才会公开，请留意审核结果通知',
    rejectedBanner: '本文已被打回，请根据下方审核意见修改后重新提交',
    resubmit: '重新提交审核',
    modeSplit: '分屏预览',
    modeIR: '即时渲染',
    modeSource: '纯源码',
    cover: '封面图',
    coverUpload: '上传封面',
    coverFromBody: '从正文选择',
    coverClear: '移除封面',
    restoreDraft: '检测到未保存的草稿，是否恢复？',
    restoreDraftConfirm: '恢复',
    restoreDraftDiscard: '放弃',
    savedAt: '已保存 {time}',
    publishing: '发布中…',
    publishUpdate: '更新',
    toc: '目录',
    metaPanel: '发布选项',
  },
  imgUpload: {
    uploadLocal: '上传本地图片',
    localHint: '上传到服务器，生成本地链接',
    uploaded: '图片已上传',
    failed: '图片上传失败',
    tooLarge: '文件过大，最大 2MB',
    badFormat: '仅支持 JPG / PNG / WEBP / GIF',
    noFile: '请选择图片文件',
  },
  rev: {
    title: '历史版本',
    empty: '还没有历史版本（首次保存后开始记录）',
    restore: '恢复此版本',
    confirm: '用这个版本覆盖当前内容？当前内容也会存进历史，可以再回退。',
    restored: '已恢复到该版本',
    chars: '字符',
  },
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
  },
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
  profile: {
    title: '个人资料',
    displayName: '显示名称',
    bio: '简介',
    avatar: '头像',
    avatarHint: '上传图片链接（本地或图床）',
    save: '保存',
    saved: '资料已保存',
    saveFailed: '资料保存失败',
  },
  imageLib: {
    title: '图片库',
    sub: '浏览并删除已上传到本站的图片',
    upload: '上传图片',
    empty: '暂无已上传的图片',
    delete: '删除',
    confirm: '确定删除这张图片吗？正在引用它的文章将显示破图。',
    deleted: '图片已删除',
    deleteFailed: '图片删除失败',
    loading: '加载中…',
    size: '大小',
    modified: '修改时间',
  },
  author: {
    title: '作者主页',
    posts: '他的文章',
    none: '该作者还没有发布文章',
  },
  register: {
    closed: '注册已关闭',
    contactAdmin: '本站暂不开放注册，如需账号请联系管理员。',
  },
  read: {
    font: '字号',
    fontSmall: '小',
    fontMedium: '中',
    fontLarge: '大',
    fontXlarge: '特大',
    reset: '重置',
    minutes: '分钟',
    read: '约 {n} 分钟读完',
    copyCode: '复制代码',
    codeCopied: '已复制',
    tocTitle: '本文目录',
    related: '相关文章',
    home: '首页',
    permanentLink: '复制永久链接',
    linkCopied: '已复制',
    tocOpen: '展开',
    tocClose: '收起',
    backToTop: '回到顶部',
    lightboxClose: '关闭预览',
  },
  notfound: {
    text: '页面不存在',
    home: '回首页',
  },
  archive: {
    title: '归档',
    empty: '暂无已发布文章',
    emptyLoggedOut: '登录后查看归档',
    loadFailed: '归档加载失败',
    total: '共 {n} 篇',
  },
  common: {
    loading: '加载中…',
    count: '{from}–{to} / {total}',
  },
}

const en: Texts = {
  nav: { list: 'All Posts', create: 'New Post', archive: 'Archive', settings: 'Settings', admin: 'Admin', backToContent: 'Back to content', dashboard: 'Dashboard', site: 'Site', groupContent: 'Content', groupUsers: 'Users', groupSystem: 'System' },
  menu: { profile: 'Profile', account: 'Account settings', logout: 'Sign out', loginOrRegister: 'Sign in / Register', confirmLogout: 'Sign out?' },
  center: {
    title: 'Personal Center',
    myHome: 'My Homepage',
    contentManage: 'Content',
    resubmitted: 'Resubmitted for review',
    resubmitFailed: 'Failed to resubmit',
    security: 'Security',
    bannerViews: 'Total views',
    bannerPosts: 'Posts',
    registeredAt: 'Joined',
    bio: 'Bio',
    editProfile: 'Edit profile',
    postsList: 'My posts',
    sortBy: 'Sort by',
    sortTime: 'Publish time',
    sortViews: 'Views',
    orderAsc: 'Ascending',
    orderDesc: 'Descending',
    empty: 'No posts',
    manageEmpty: 'No content',
    status: 'Status',
    edit: 'Edit',
    account: 'Account',
    username: 'Username',
    maskedHint: 'Sensitive info is partially masked',
    show: 'Show',
    hide: 'Hide',
    securityEmpty: 'No account info',
    securityNote: 'Security center: change password, view login history, sign out everywhere.',
    secChangePw: 'Change password',
    secCurrentPw: 'Current password',
    secNewPw: 'New password',
    secConfirmPw: 'Confirm new password',
    secPwBtn: 'Update password',
    secPwChanged: 'Password updated',
    secPwMismatch: 'New passwords do not match',
    secPwTooShort: 'New password must be at least 8 characters',
    secPwWrong: 'Current password is incorrect',
    secLoginHistory: 'Login history',
    secNoLogins: 'No login records yet',
    secLoginOk: 'Success',
    secLoginFail: 'Failed',
    secIp: 'IP',
    secDevice: 'Device',
    secLogoutAll: 'Sign out all devices',
    secLogoutAllConfirm: 'Sign out of all devices? This will end your session here and on every other device.',
    secLogoutAllDone: 'Signed out everywhere — please sign in again',
    loading: 'Loading…',
    loadError: 'Failed to load',
    retry: 'Retry',
  },
  auth: {
    loginTitle: 'Sign In',
    loginSub: 'Sign in with username or email',
    registerTitle: 'Register',
    registerSub: 'Create an account, you will be signed in automatically',
    logout: 'Sign out',
    welcomeBack: 'Welcome back, ',
    loginFailed: 'Login failed',
    registerFailed: 'Registration failed',
    noAccount: 'No account yet?',
    hasAccount: 'Already have an account?',
    empty: 'Please fill in all fields',
    username: 'Username / Email',
    usernamePlaceholder: 'Username or email',
    password: 'Password',
    passwordPlaceholder: 'Enter your password',
    showPwd: 'Show',
    hidePwd: 'Hide',
    email: 'Email',
    loginBtn: 'Sign in',
    registering: 'Register',
    loginBusy: 'Signing in…',
    passwordTooShort: 'Password must be at least 8 characters',
  },
  password: {
    title: 'First login — change your password',
    subtitle: 'A temporary password was generated for you. Set your own password to continue.',
    alreadySet: 'No password change required. Continue.',
    continue: 'Continue',
    fillAll: 'Please fill in all fields',
    mismatch: 'The two new passwords do not match',
    tooShort: 'New password must be at least 8 characters',
    changed: 'Password changed',
    changeFailed: 'Failed to change password. Check your current password.',
    newLabel: 'New password',
    confirmLabel: 'Confirm new password',
    currentPlaceholder: 'Current (temporary) password',
    newPlaceholder: 'At least 8 characters',
    busy: 'Changing…',
    submit: 'Change password',
  },
  list: {
    title: 'Posts',
    searchPlaceholder: 'Search title / body / author…',
    allStatus: 'All status',
    published: 'Published',
    author: 'Author',
    edit: 'Edit',
    del: 'Delete',
    empty: 'No posts yet. Click the button above to write your first one.',
    emptyLoggedOut: 'Please sign in first.',
    noResults: 'No posts match these filters. Try another keyword or category.',
    clearFilters: 'Clear filters',
    deleteConfirm: 'Delete this post?',
    prevPage: 'Previous',
    nextPage: 'Next',
    pagination: 'Pagination',
    firstPage: 'First',
    lastPage: 'Last',
    jumpTo: 'Go to',
    jumpPage: 'page',
    pageOutOfRange: 'This page has no content — the link may be out of date.',
    pageOf: 'Page {page} of {total}',
    allCategories: 'All categories',
    categoryLabel: 'Category',
    tagLabel: 'Tag',
    pinned: 'Pinned',
    views: 'views',
    loadingMore: 'Loading…',
    noMore: 'No more posts',
    loadMore: 'Load more',
    private: 'Private',
    visibility: 'Visibility',
    setPrivate: 'Make private',
    setPublic: 'Make public',
    visibilityUpdated: 'Visibility updated',
    visibilityUndo: 'Undo',
    visibilityFailed: 'Failed to update visibility',
  },
  sidebar: {
    categories: 'Categories',
    tags: 'Tags',
    recent: 'Recent Posts',
    allCategories: 'All categories',
  },
  post: {
    loading: 'Loading…',
    notFound: 'Post not found',
    backToList: 'Back to list',
    created: 'Created',
    updated: 'Updated',
    deleted: 'Deleted',
    deleteConfirm: 'Delete "{t}"?',
    loadFailed: 'Failed to load',
    emptyBody: '(no body)',
    words: 'words',
    views: 'views',
    prevPost: 'Previous post',
    nextPost: 'Next post',
    pinned: 'Pinned',
  },
  editor: {
    title: 'Title',
    body: 'Body (Markdown)',
    status: 'Status',
    category: 'Category',
    tags: 'Tags',
    tagPlaceholder: 'Enter tags, comma separated…',
    allCategories: 'All categories',
    publish: 'Publish',
    cancel: 'Cancel',
    save: 'Save changes',
    created: 'Post created',
    saved: 'Saved',
    createFailed: 'Failed to create',
    saveFailed: 'Failed to save',
    titlePlaceholder: 'Post title',
    heading: 'Write a post',
    bodyPlaceholder: 'Write your content here…',
    preview: 'Preview',
    editBtn: 'Edit',
    h1: 'Heading 1',
    h2: 'Heading 2',
    h3: 'Heading 3',
    bold: 'Bold',
    italic: 'Italic',
    code: 'Code',
    blockquote: 'Quote',
    link: 'Link',
    image: 'Image',
    list: 'List',
    autosaveSaved: 'Auto-saved',
    pinnedLabel: 'Pin to top',
    pinnedHint: 'Pinned posts always appear at the top of the list',
    dragDivider: 'Drag to adjust split',
    stats: '{n} words',
    readTime: '~ {n} min read',
    visibility: 'Visibility',
    visibilityPublic: 'Public',
    visibilityPrivate: 'Private',
    visibilityPublicHint: 'Anyone can read this post, even when logged out',
    visibilityPrivateHint: 'Only you can see it; others get a 404',
    visibilityReviewHint: 'This post becomes public only after it passes review',
    rejectedBanner: 'This post was rejected. Revise it using the review note below, then resubmit',
    resubmit: 'Resubmit for review',
    modeSplit: 'Split preview',
    modeIR: 'Instant render',
    modeSource: 'Source',
    cover: 'Cover',
    coverUpload: 'Upload cover',
    coverFromBody: 'Pick from content',
    coverClear: 'Remove cover',
    restoreDraft: 'Found an unsaved draft. Restore it?',
    restoreDraftConfirm: 'Restore',
    restoreDraftDiscard: 'Discard',
    savedAt: 'Saved {time}',
    publishing: 'Publishing…',
    publishUpdate: 'Update',
    toc: 'Outline',
    metaPanel: 'Publish options',
  },
  imgUpload: {
    uploadLocal: 'Upload local image',
    localHint: 'Upload to server, generates a /uploads/ link',
    uploaded: 'Image uploaded',
    failed: 'Failed to upload image',
    tooLarge: 'File too large, max 2 MB',
    badFormat: 'Only JPG / PNG / WEBP / GIF are allowed',
    noFile: 'Please select an image file',
  },
  rev: {
    title: 'Revision history',
    empty: 'No earlier versions yet (recorded from the first save)',
    restore: 'Restore this version',
    confirm: 'Overwrite the current content with this version? The current text is kept in history too.',
    restored: 'Version restored',
    chars: 'chars',
  },
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
  },
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
  profile: {
    title: 'Profile',
    displayName: 'Display name',
    bio: 'Bio',
    avatar: 'Avatar',
    avatarHint: 'Paste an image URL (local or image host)',
    save: 'Save',
    saved: 'Profile saved',
    saveFailed: 'Failed to save profile',
  },
  imageLib: {
    title: 'Image library',
    sub: 'Browse and delete images uploaded to this site',
    upload: 'Upload image',
    empty: 'No uploaded images yet',
    delete: 'Delete',
    confirm: 'Delete this image? Posts using it will show a broken image.',
    deleted: 'Image deleted',
    deleteFailed: 'Failed to delete image',
    loading: 'Loading…',
    size: 'Size',
    modified: 'Modified',
  },
  author: {
    title: 'Author',
    posts: 'Posts',
    none: 'This author has not published anything yet',
  },
  register: {
    closed: 'Registration closed',
    contactAdmin: 'Public registration is disabled. Contact the admin for an account.',
  },
  read: {
    font: 'Font size',
    fontSmall: 'Small',
    fontMedium: 'Medium',
    fontLarge: 'Large',
    fontXlarge: 'Extra large',
    reset: 'Reset',
    minutes: 'min',
    read: '~ {n} min read',
    copyCode: 'Copy code',
    codeCopied: 'Copied',
    tocTitle: 'On this page',
    related: 'Related posts',
    home: 'Home',
    permanentLink: 'Copy permalink',
    linkCopied: 'Copied',
    tocOpen: 'Expand',
    tocClose: 'Collapse',
    backToTop: 'Back to top',
    lightboxClose: 'Close preview',
  },
  archive: {
    title: 'Archive',
    empty: 'No published posts yet.',
    emptyLoggedOut: 'Sign in to view the archive.',
    loadFailed: 'Failed to load archive',
    total: '{n} posts',
  },
  notfound: {
    text: 'Page not found',
    home: 'Go home',
  },
  common: {
    loading: 'Loading…',
    count: '{from}–{to} / {total}',
  },
}

const dicts: Record<Locale, Texts> = { zh, en }
const LOCALE_KEY = 'echo.locale'

function detect(): Locale {
  const saved = localStorage.getItem(LOCALE_KEY) as Locale | null
  if (saved === 'zh' || saved === 'en') return saved
  return navigator.language?.toLowerCase().startsWith('en') ? 'en' : 'zh'
}

type T = (key: string, vars?: Record<string, string>) => string

interface I18nValue {
  locale: Locale
  setLocale: (l: Locale) => void
  t: T
}

const I18nContext = createContext<I18nValue | null>(null)

export function I18nProvider({ children }: { children: ComponentChildren }) {
  const [locale, setLocale] = useState<Locale>(detect)

  useEffect(() => {
    document.documentElement.lang = locale
  }, [locale])

  const apply = (l: Locale) => {
    localStorage.setItem(LOCALE_KEY, l)
    setLocale(l)
  }

  const t: T = (key, vars) => {
    const parts = key.split('.')
    let node: unknown = dicts[locale]
    for (const p of parts) {
      node = (node as Record<string, unknown>)?.[p]
    }
    let out = typeof node === 'string' ? node : key
    if (vars) for (const [k, v] of Object.entries(vars)) out = out.replace(`{${k}}`, v)
    return out
  }

  return (
    <I18nContext.Provider value={{ locale, setLocale: apply, t }}>
      {children}
    </I18nContext.Provider>
  )
}

export function useI18n(): I18nValue {
  const v = useContext(I18nContext)
  if (!v) throw new Error('useI18n must be used within <I18nProvider>')
  return v
}

// Editor toolbar, image upload and revision history.

export type EditorTexts = {
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
  },
  imgUpload: {
    uploadLocal: string
    localHint: string
    uploaded: string
    failed: string
    tooLarge: string
    badFormat: string
    noFile: string
  },
  rev: {
    title: string
    empty: string
    restore: string
    confirm: string
    restored: string
    chars: string
  }
}

export const editorZh: EditorTexts = {
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
  }
}

export const editorEn: EditorTexts = {
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
  }
}

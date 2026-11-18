// Editor toolbar, image upload and revision history.

export type EditorTexts = {
  editor: {
    title: string
    body: string
    status: string
    category: string
    tags: string
    tagPlaceholder: string
    tagCreate: string
    tagHint: string
    tagRemove: string
    allCategories: string
    publish: string
    cancel: string
    save: string
    created: string
    saved: string
    createFailed: string
    saveFailed: string
    conflictReload: string
    notYourPost: string
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
    rereviewHint: string
    pendingHint: string
    submitReview: string
    savePrivate: string
    saving: string
    rejectedBanner: string
    resubmit: string
    modeSplit: string
    modeIR: string
    modeSource: string
    formatMarkdown: string
    formatHTML: string
    htmlPlaceholder: string
    uploadPdf: string
    pdfUploading: string
    pdfUploadFailed: string
    attachmentLimit: string
    attachmentRemove: string
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
    publishUpdateRequeue: string
    requeued: string
    toc: string
    metaPanel: string
    settings: string
    settingsTitle: string
    settingsGeneral: string
    settingsAdvanced: string
    settingsDone: string
    settingsClose: string
    slug: string
    slugPlaceholder: string
    slugHint: string
  },
  imgUpload: {
    uploadLocal: string
    localHint: string
    uploaded: string
    failed: string
    tooLarge: string
    badFormat: string
    noFile: string
 // The library already held these exact bytes.
    deduped: string
    // Avatar picker (crop + resize happens in the browser).
    cropTitle: string
    cropHint: string
    cropApply: string
    cropCancel: string
  },
  rev: {
    title: string
    empty: string
    restore: string
    confirm: string
    restored: string
    chars: string
  }
  // Image upload size/compression dialog (shared by the editor and the
  // attachment library).
  uploadOpts: {
    title: string
    original: string
    custom: string
    maxWidth: string
    maxHeight: string
    quality: string
    lossless: string
    losslessHint: string
    pngHint: string
    unsupported: string
    ratioHint: string
    target: string
    keepOriginal: string
    remember: string
    apply: string
    cancel: string
  }
}

export const editorZh: EditorTexts = {
  editor: {
    title: '标题',
    body: '正文（支持 Markdown）',
    status: '状态',
    category: '分类',
    tags: '标签',
    tagPlaceholder: '搜索或输入新标签…',
    tagCreate: '创建标签',
    tagHint: '输入即搜索已有标签，↑↓ 选择，回车确认；没有的标签会给出创建入口。',
    tagRemove: '移除标签',
    allCategories: '全部分类',
    publish: '发布',
    cancel: '取消',
    save: '保存修改',
    created: '文章已创建',
    saved: '已保存',
    createFailed: '创建失败',
    conflictReload: '这篇文章刚被审核或管理操作改动过，你这次保存没有生效。请刷新看清最新状态后再改',
    saveFailed: '保存失败',
    notYourPost: '只有作者本人或 root 可以修改这篇文章的标题与正文。你可以把它退回给作者并附上修改意见。',
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
    visibilityPublicHint: '保存后所有读者都能看到这篇文章',
    visibilityPrivateHint: '不出现在公开列表，游客和其他读者访问会显示 404；网站管理员可以看到。不进审核队列，随时可以再提交审核',
    visibilityReviewHint: '提交后进入审核队列，审核通过才会公开',
    rereviewHint: '修改正文、标题或封面后，这篇文章会重新进入审核队列；审核通过前，它不会显示在公开页面。只改分类、标签或链接不会触发审核',
    pendingHint: '这篇文章正在审核队列中，审核通过前不会公开；再次保存会保持待审状态',
    submitReview: '提交审核',
    savePrivate: '保存',
    saving: '保存中…',
    rejectedBanner: '本文已被打回，请根据下方审核意见修改后重新提交',
    resubmit: '重新提交审核',
    modeSplit: '分屏预览',
    modeIR: '即时渲染',
    modeSource: '纯源码',
    formatMarkdown: 'Markdown',
    formatHTML: 'HTML',
    htmlPlaceholder: '在这里写 HTML 片段（不要写 <html>/<head>/<body> 外壳；脚本、样式、iframe 会被服务端移除）',
    uploadPdf: 'PDF 附件',
    pdfUploading: '上传中…',
    pdfUploadFailed: 'PDF 上传失败',
    attachmentLimit: '每篇文章最多 10 个 PDF 附件',
    attachmentRemove: '移除附件',
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
    publishUpdateRequeue: '保存并重新送审',
    requeued: '已提交审核，审核通过前文章暂不对外显示',
    toc: '目录',
    metaPanel: '发布选项',
    settings: '设置',
    settingsTitle: '发布设置',
    settingsGeneral: '常规',
    settingsAdvanced: '高级',
    settingsDone: '完成',
    settingsClose: '关闭设置',
    slug: '链接别名',
    slugPlaceholder: '留空自动生成',
    slugHint: '留空时：新建文章由标题生成，编辑文章保持当前链接不变。修改后旧链接会失效。',
  },
  imgUpload: {
    uploadLocal: '上传本地图片',
    localHint: '上传到服务器，生成本地链接',
    uploaded: '图片已上传',
    failed: '图片上传失败',
    tooLarge: '文件超过本站允许的大小',
    badFormat: '仅支持 JPG / PNG / WEBP / GIF',
    noFile: '请选择图片文件',
    deduped: '图库中已有相同图片，已直接复用（未重复占用空间）',
    cropTitle: '调整头像',
    cropHint: '拖动图片调整位置，滚轮或滑块缩放。头像会按 1:1 裁剪。',
    cropApply: '使用这张',
    cropCancel: '取消',
  },
  rev: {
    title: '历史版本',
    empty: '还没有历史版本（首次保存后开始记录）',
    restore: '恢复此版本',
    confirm: '用这个版本覆盖当前内容？当前内容也会存进历史，可以再回退。',
    restored: '已恢复到该版本',
    chars: '字符',
  },
  uploadOpts: {
    title: '图片上传设置',
    original: '原图',
    custom: '自定义',
    maxWidth: '最大宽度',
    maxHeight: '最大高度',
    quality: 'JPEG 质量',
    lossless: '最高质量（100）',
    losslessHint: '质量 100 的 JPEG 体积明显变大，但仍是有损编码；只有 PNG 才是像素无损。',
    pngHint: 'PNG 使用无损压缩，质量设置不影响它。',
    unsupported: 'WebP / GIF 不支持服务端缩放；GIF 缩放还会丢失动画。请选择原图上传。',
    ratioHint: '宽高是一个边界框：图片会等比缩放进去，不会变形，也不会放大。',
    target: '输出尺寸',
    keepOriginal: '保持原图尺寸',
    remember: '记住这次设置，多图上传时直接使用',
    apply: '上传',
    cancel: '取消',
  }
}

export const editorEn: EditorTexts = {
  editor: {
    title: 'Title',
    body: 'Body (Markdown)',
    status: 'Status',
    category: 'Category',
    tags: 'Tags',
    tagPlaceholder: 'Search or type a new tag…',
    tagCreate: 'Create tag',
    tagHint: 'Typing searches existing tags — ↑↓ to pick, Enter to confirm. Tags that do not exist yet get a create row.',
    tagRemove: 'Remove tag',
    allCategories: 'All categories',
    publish: 'Publish',
    cancel: 'Cancel',
    save: 'Save changes',
    created: 'Post created',
    saved: 'Saved',
    createFailed: 'Failed to create',
    conflictReload: 'This post was just reviewed or moderated, so this save did not apply. Reload to see its current state, then edit again',
    saveFailed: 'Failed to save',
    notYourPost: 'Only the author or the root can change this post\u2019s title and body. Send it back to the author with a review note instead.',
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
    visibilityPublicHint: 'Once saved, anyone can read this post',
    visibilityPrivateHint: 'Hidden from public listings: visitors and other readers get a 404, while site admins can read it. Private posts skip review and you can submit one for review at any time',
    visibilityReviewHint: 'Submitting puts it in the review queue; it goes public only after it passes',
    rereviewHint: 'Changing the text, title or cover sends this article back through review; until it passes again it will not appear on the public site. Editing only the category, tags or slug does not trigger a review',
    pendingHint: 'This article is in the review queue and stays hidden until it passes; saving again keeps it queued',
    submitReview: 'Submit for review',
    savePrivate: 'Save',
    saving: 'Saving…',
    rejectedBanner: 'This post was rejected. Revise it using the review note below, then resubmit',
    resubmit: 'Resubmit for review',
    modeSplit: 'Split preview',
    modeIR: 'Instant render',
    modeSource: 'Source',
    formatMarkdown: 'Markdown',
    formatHTML: 'HTML',
    htmlPlaceholder: 'Write an HTML fragment here (no <html>/<head>/<body> shell; scripts, styles and iframes are removed server-side)',
    uploadPdf: 'PDF attachment',
    pdfUploading: 'Uploading…',
    pdfUploadFailed: 'Failed to upload PDF',
    attachmentLimit: 'A post can carry at most 10 PDF attachments',
    attachmentRemove: 'Remove attachment',
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
    publishUpdateRequeue: 'Save and resubmit for review',
    requeued: 'Submitted for review — the article stays hidden until it passes',
    toc: 'Outline',
    metaPanel: 'Publish options',
    settings: 'Settings',
    settingsTitle: 'Publish settings',
    settingsGeneral: 'General',
    settingsAdvanced: 'Advanced',
    settingsDone: 'Done',
    settingsClose: 'Close settings',
    slug: 'Slug',
    slugPlaceholder: 'Leave empty to generate',
    slugHint: 'Left empty: derived from the title on create, and left unchanged on edit. Changing it breaks the old link.',
  },
  imgUpload: {
    uploadLocal: 'Upload local image',
    localHint: 'Upload to server, generates a /uploads/ link',
    uploaded: 'Image uploaded',
    failed: 'Failed to upload image',
    tooLarge: 'File exceeds the size this site allows',
    badFormat: 'Only JPG / PNG / WEBP / GIF are allowed',
    noFile: 'Please select an image file',
    deduped: 'This image is already in the library — reusing it, nothing new was stored',
    cropTitle: 'Adjust avatar',
    cropHint: 'Drag to position, scroll or use the slider to zoom. The avatar is cropped 1:1.',
    cropApply: 'Use this',
    cropCancel: 'Cancel',
  },
  rev: {
    title: 'Revision history',
    empty: 'No earlier versions yet (recorded from the first save)',
    restore: 'Restore this version',
    confirm: 'Overwrite the current content with this version? The current text is kept in history too.',
    restored: 'Version restored',
    chars: 'chars',
  },
  uploadOpts: {
    title: 'Image upload settings',
    original: 'Original',
    custom: 'Custom',
    maxWidth: 'Max width',
    maxHeight: 'Max height',
    quality: 'JPEG quality',
    lossless: 'Highest quality (100)',
    losslessHint: 'Quality 100 makes JPEG files noticeably larger, but JPEG stays lossy; only PNG is pixel-lossless.',
    pngHint: 'PNG uses lossless compression; the quality setting does not apply.',
    unsupported: 'WebP / GIF cannot be resized on the server, and resizing a GIF would drop its animation. Upload the original.',
    ratioHint: 'Width and height are a bounding box: the image scales down proportionally, is never distorted, and is never enlarged.',
    target: 'Output size',
    keepOriginal: 'Keep the original size',
    remember: 'Remember these settings for multi-image uploads',
    apply: 'Upload',
    cancel: 'Cancel',
  }
}

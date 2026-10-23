// Reader-facing pages: list, post, read, archive, author, 404.

export type ReaderTexts = {
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
    allTags: string
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
  },
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
  },
  author: {
    title: string
    posts: string
    none: string
  },
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
  },
  notfound: {
    text: string
    home: string
  },
  archive: {
    title: string
    empty: string
    emptyLoggedOut: string
    loadFailed: string
    total: string
  }
}

export const readerZh: ReaderTexts = {
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
    allTags: '全部标签',
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
  author: {
    title: '作者主页',
    posts: '他的文章',
    none: '该作者还没有发布文章',
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
  }
}

export const readerEn: ReaderTexts = {
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
    allTags: 'All tags',
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
  author: {
    title: 'Author',
    posts: 'Posts',
    none: 'This author has not published anything yet',
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
  notfound: {
    text: 'Page not found',
    home: 'Go home',
  },
  archive: {
    title: 'Archive',
    empty: 'No published posts yet.',
    emptyLoggedOut: 'Sign in to view the archive.',
    loadFailed: 'Failed to load archive',
    total: '{n} posts',
  }
}

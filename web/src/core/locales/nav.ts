// Navigation, menu and sidebar labels.

export type NavTexts = {
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
  },
  menu: {
    profile: string
    account: string
    logout: string
    loginOrRegister: string
    confirmLogout: string
  },
  sidebar: {
    categories: string
    tags: string
    recent: string
    allCategories: string
  }
}

export const navZh: NavTexts = {
  nav: { list: '全部文章', create: '写文章', archive: '归档', settings: '设置', admin: '管理', backToContent: '返回内容', dashboard: '仪表盘', site: '站点设置', groupContent: '内容', groupUsers: '用户', groupSystem: '系统' },
  menu: { profile: '个人中心', account: '账号设置', logout: '退出登录', loginOrRegister: '登录 / 注册', confirmLogout: '确定退出登录吗？' },
  sidebar: {
    categories: '分类',
    tags: '标签',
    recent: '最近文章',
    allCategories: '全部分类',
  }
}

export const navEn: NavTexts = {
  nav: { list: 'All Posts', create: 'New Post', archive: 'Archive', settings: 'Settings', admin: 'Admin', backToContent: 'Back to content', dashboard: 'Dashboard', site: 'Site', groupContent: 'Content', groupUsers: 'Users', groupSystem: 'System' },
  menu: { profile: 'Profile', account: 'Account settings', logout: 'Sign out', loginOrRegister: 'Sign in / Register', confirmLogout: 'Sign out?' },
  sidebar: {
    categories: 'Categories',
    tags: 'Tags',
    recent: 'Recent Posts',
    allCategories: 'All categories',
  }
}

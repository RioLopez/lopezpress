// Personal center (my homepage / content / security).

export type CenterTexts = {
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
    secPwTooLong: string
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
}

export const centerZh: CenterTexts = {
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
    secPwTooShort: '新密码至少 8 位，且不要用常见密码',
    secPwTooLong: '新密码太长（最多 72 字节，约 24 个汉字）',
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
  }
}

export const centerEn: CenterTexts = {
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
    secPwTooShort: 'At least 8 characters, and not a common password',
    secPwTooLong: 'Too long (max 72 bytes, about 24 Chinese characters)',
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
  }
}

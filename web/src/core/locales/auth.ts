// Authentication, password change, registration and profile.

export type AuthTexts = {
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
  },
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
  },
  profile: {
    title: string
    displayName: string
    bio: string
    avatar: string
    avatarHint: string
    save: string
    saved: string
    saveFailed: string
  },
  register: {
    closed: string
    contactAdmin: string
  }
}

export const authZh: AuthTexts = {
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
  register: {
    closed: '注册已关闭',
    contactAdmin: '本站暂不开放注册，如需账号请联系管理员。',
  }
}

export const authEn: AuthTexts = {
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
  register: {
    closed: 'Registration closed',
    contactAdmin: 'Public registration is disabled. Contact the admin for an account.',
  }
}

import { render } from 'preact'
import { Route, Switch, Redirect, useLocation } from 'wouter'
import './index.css'
import type { Role } from './core/types'
import { AuthProvider, useAuth } from './core/auth'
import { AppProvider } from './core/app'
import { I18nProvider, useI18n } from './core/i18n'
import { ThemeProvider } from './core/theme'
import { SiteProvider, useSite } from './core/site'
import { Layout } from './ui/Layout'
import { AdminLayout } from './ui/AdminLayout'
import { Toasts } from './ui/Toast'
import { LoginPage } from './pages/LoginPage'
import { RegisterPage } from './pages/RegisterPage'
import { ForgotPasswordPage } from './pages/ForgotPasswordPage'
import { ChangePasswordPage } from './pages/ChangePasswordPage'
import { ListPage } from './pages/ListPage'
import { ArchivePage } from './pages/ArchivePage'
import { CreatePage } from './pages/CreatePage'
import { EditPage } from './pages/EditPage'
import { ReadPage } from './pages/ReadPage'
import { SlugPage } from './pages/SlugPage'
import { SettingsPage } from './pages/SettingsPage'
import { PersonalCenterPage } from './pages/PersonalCenterPage'
import { NotFoundPage } from './pages/NotFoundPage'
import { AdminCategoriesPage } from './pages/AdminCategoriesPage'
import { AdminPostsPage } from './pages/AdminPostsPage'
import { AdminTagsPage } from './pages/AdminTagsPage'
import { AdminUsersPage } from './pages/AdminUsersPage'
import { AdminBackupPage } from './pages/AdminBackupPage'
import { AdminMediaPage } from './pages/AdminMediaPage'
import { AdminTrashPage } from './pages/AdminTrashPage'
import { AdminAuditPage } from './pages/AdminAuditPage'
import { AdminDashboardPage } from './pages/AdminDashboardPage'
import { AdminSitePage } from './pages/AdminSitePage'
import { AuthorPage } from './pages/AuthorPage'
import { AdminReviewPage } from './pages/AdminReviewPage'
import { AdminBanVaultPage } from './pages/AdminBanVaultPage'

// Authed content pages redirect to /login when there is no JWT.
function Guarded({ children }: { children: any }) {
  const { hasToken } = useAuth()
  if (!hasToken) return <Redirect to="/login" />
  return children as any
}

// Spec 27: reader-facing routes (list / archive / article) are readable
// anonymously ONLY when the admin enabled site_config.public_read. Authoring
// routes keep using Guarded — a signed-out visitor can never reach them.
// The API enforces the same switch (anonymous reads 401 when locked), so this
// guard is UX, not the security boundary.
function Readable({ children }: { children: any }) {
  const { hasToken } = useAuth()
  const { config } = useSite()
  if (hasToken || config.public_read) return children as any
  return <Redirect to="/login" />
}

// Admin pages: require a JWT *and* a role in the allow-list, then render
// inside the left-vertical sidebar shell (spec 06). Root is the super-admin;
// admins manage day-to-day. The allow-list lives HERE (the routing layer) —
// pages must not re-check roles (ARCH_REVIEW F3), which is what caused the
// duplicated `user?.role !== 'admin' && ...` guards across admin pages.
function AdminGuarded({ children, roles = ['admin', 'root'] }: { children: any; roles?: Role[] }) {
  const { user, hasToken } = useAuth()
  const { t } = useI18n()
  if (!hasToken) return <Redirect to="/login" />
  if (!user || !roles.includes(user.role)) {
    const denied = roles.length === 1 && roles[0] === 'root' ? t('admin.rootOnly') : t('admin.notAdmin')
    return (
      <Layout>
        <div class="admin-page">
          <p class="muted">{denied}</p>
        </div>
      </Layout>
    )
  }
  return <AdminLayout>{children}</AdminLayout>
}

// /login and /register: already signed in? Go straight to the list.
function GuestOnly({ children }: { children: any }) {
  const { hasToken } = useAuth()
  if (hasToken) return <Redirect to="/" />
  return children as any
}

function AppView() {
  // Admin routes render inside their own full-screen shell (AdminLayout);
  // wrapping them in the content Layout as well would double the top bar and
  // footer. So for /admin* paths we render the Switch without the content
  // Layout; every other route keeps it.
  const [path] = useLocation()
  const isAdmin = path === '/admin' || path.startsWith('/admin/')
  const routes = (
    <Switch>
      {/* Switch makes the routes mutually exclusive: without it every
          matching Route renders at once, so the catch-all NotFoundPage below
          would appear under every page. */}
      <Route
        path="/"
        component={() => <Readable><ListPage /></Readable>}
      />
      <Route
        path="/archive"
        component={() => <Readable><ArchivePage /></Readable>}
      />
      <Route
        path="/create"
        component={() => <Guarded><CreatePage /></Guarded>}
      />
      <Route
        path="/edit/:id"
        component={(p: any) => (
          <Guarded><EditPage id={p.params.id} /></Guarded>
        )}
      />
      <Route
        path="/read/:id"
        component={(p: any) => (
          <Readable><ReadPage id={p.params.id} /></Readable>
        )}
      />
      {/* Spec 29: permanent links. /read/{uuid} stays valid forever, so both
          shapes address the same post. */}
      <Route
        path="/p/:slug"
        component={(p: any) => (
          <Readable><SlugPage slug={p.params.slug} /></Readable>
        )}
      />
      <Route
        path="/settings"
        component={() => <Guarded><SettingsPage /></Guarded>}
      />
      {/* Personal center (spec 20): my homepage / content / security. */}
      <Route
        path="/me"
        component={() => <Guarded><PersonalCenterPage /></Guarded>}
      />
      {/* Public author profile — readable without signing in. */}
      <Route
        path="/u/:username"
        component={(p: any) => <AuthorPage username={p.params.username} />}
      />

      {/* Admin: left vertical sidebar shell + admin-role gate. */}
      <Route
        path="/admin"
        component={() => <AdminGuarded><AdminDashboardPage /></AdminGuarded>}
      />
      <Route
        path="/admin/posts"
        component={() => <AdminGuarded><AdminPostsPage /></AdminGuarded>}
      />
      <Route
        path="/admin/categories"
        component={() => <AdminGuarded><AdminCategoriesPage /></AdminGuarded>}
      />
      <Route
        path="/admin/tags"
        component={() => <AdminGuarded><AdminTagsPage /></AdminGuarded>}
      />
      <Route
        path="/admin/users"
        component={() => <AdminGuarded><AdminUsersPage /></AdminGuarded>}
      />
      <Route
        path="/admin/site"
        component={() => <AdminGuarded><AdminSitePage /></AdminGuarded>}
      />
      <Route
        path="/admin/media"
        component={() => <AdminGuarded><AdminMediaPage /></AdminGuarded>}
      />
      <Route
        path="/admin/backup"
        component={() => <AdminGuarded><AdminBackupPage /></AdminGuarded>}
      />
      <Route
        path="/admin/trash"
        component={() => <AdminGuarded><AdminTrashPage /></AdminGuarded>}
      />
      <Route
        path="/admin/audit"
        component={() => <AdminGuarded><AdminAuditPage /></AdminGuarded>}
      />
      {/* Content review workflow (spec 24): pending queue for admin/root. */}
      <Route
        path="/admin/review"
        component={() => <AdminGuarded><AdminReviewPage /></AdminGuarded>}
      />
      {/* Ban vault (spec 24): root-only list of banned posts. */}
      <Route
        path="/admin/banned"
        component={() => <AdminGuarded roles={['root']}><AdminBanVaultPage /></AdminGuarded>}
      />

      <Route
        path="/login"
        component={() => <GuestOnly><LoginPage /></GuestOnly>}
      />
      <Route
        path="/register"
        component={() => <GuestOnly><RegisterPage /></GuestOnly>}
      />
      {/* Self-service password reset (spec 37, experimental). GuestOnly: a
          signed-in user changes the password from the settings page instead,
          where the current one is required. */}
      <Route
        path="/forgot-password"
        component={() => <GuestOnly><ForgotPasswordPage /></GuestOnly>}
      />
      {/* Forced first-login password change (first-boot admin, spec #2).
          Guarded but NOT GuestOnly: the user must be signed in, and if the
          flag is already cleared the page just hands off to "/" cleanly. */}
      <Route
        path="/change-password"
        component={() => <Guarded><ChangePasswordPage /></Guarded>}
      />
      <Route component={() => <NotFoundPage />} />
      </Switch>
  )
  return (
    <>
      {isAdmin ? routes : <Layout>{routes}</Layout>}
      <Toasts />
    </>
  )
}

export const App = () => {
  return (
    <ThemeProvider>
      <I18nProvider>
        <AuthProvider>
          <AppProvider>
            <SiteProvider>
              <AppView />
            </SiteProvider>
          </AppProvider>
        </AuthProvider>
      </I18nProvider>
    </ThemeProvider>
  )
}

render(<App />, document.getElementById('app') as HTMLElement)

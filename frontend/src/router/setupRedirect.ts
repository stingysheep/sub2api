export function resolveCompletedSetupRedirectPath(isAuthenticated: boolean, isAdmin: boolean, isOperator = false): string {
  if (!isAuthenticated) {
    return '/login'
  }

  return isAdmin ? '/admin/dashboard' : isOperator ? '/operator/users' : '/dashboard'
}

export type NavItem = {
  href: string;
  label: string;
  description: string;
  roles?: string[];
};

const readableRoles = ['owner', 'viewer', 'read_only', 'read-only'];

export const navItems: NavItem[] = [
  { href: '/', label: 'Dashboard', description: 'Local ocserv status overview', roles: readableRoles },
  { href: '/users', label: 'Users', description: 'Password and certificate users', roles: ['owner'] },
  { href: '/sessions', label: 'Sessions', description: 'Active session visibility and control', roles: ['owner'] },
  { href: '/history', label: 'History', description: 'Historical session analytics', roles: readableRoles },
  { href: '/config', label: 'Config', description: 'Validated configuration editing', roles: ['owner'] },
  { href: '/certificates', label: 'Certificates', description: 'Certificate visibility and lifecycle', roles: ['owner'] },
  { href: '/audit', label: 'Audit', description: 'Administrative action history', roles: ['owner'] },
  { href: '/settings', label: 'Settings', description: 'Local panel settings', roles: ['owner'] }
];

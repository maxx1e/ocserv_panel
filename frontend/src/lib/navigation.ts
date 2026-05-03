// frontend/src/lib/navigation.ts
import {
  LayoutDashboard,
  Users,
  Link,
  History,
  Settings,
  ShieldCheck,
  FileText,
  ClipboardList
} from 'lucide-svelte';

export type NavItem = {
  href: string;
  label: string;
  description: string;
  roles?: string[];
  icon: typeof LayoutDashboard;
  section: 'main' | 'management' | 'system';
};

const readableRoles = ['owner', 'viewer', 'read_only', 'read-only'];

export const navItems: NavItem[] = [
  { href: '/', label: 'Dashboard', description: 'Local ocserv status overview', icon: LayoutDashboard, section: 'main', roles: readableRoles },
  { href: '/users', label: 'Users', description: 'Password and certificate users', icon: Users, section: 'main', roles: ['owner'] },
  { href: '/sessions', label: 'Sessions', description: 'Active session visibility and control', icon: Link, section: 'main', roles: ['owner'] },
  { href: '/history', label: 'History', description: 'Historical session analytics', icon: History, section: 'main', roles: readableRoles },
  { href: '/config', label: 'Config', description: 'Validated configuration editing', icon: Settings, section: 'management', roles: ['owner'] },
  { href: '/certificates', label: 'Certificates', description: 'Certificate visibility and lifecycle', icon: ShieldCheck, section: 'management', roles: ['owner'] },
  { href: '/audit', label: 'Audit', description: 'Administrative action history', icon: FileText, section: 'management', roles: ['owner'] },
  { href: '/settings', label: 'Settings', description: 'Local panel settings', icon: ClipboardList, section: 'system', roles: ['owner'] }
];

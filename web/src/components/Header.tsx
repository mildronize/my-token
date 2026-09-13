// Ported from my-task (origin/main @ fdceb8befbcebfa25d2216d71264bb7c5e8c96d7,
// src/components/Header.tsx) per this milestone's bucket-2 rule: routing
// only, nothing else changes. next/link's `Link` -> react-router's `Link`
// (whose navigation prop is named `to`, not `href` — the one mechanical
// consequence of swapping components, not a logic change), and
// next/navigation's `usePathname` -> react-router's `useLocation().pathname`.
//
// story-1/ticket-16 deleted the example domain module (and its own
// nav links along with it) — the usage console (story-1/ticket-14) is
// this app's only real remaining screen. Still no "Projects" — that
// domain concept genuinely doesn't exist here (GOAL.md's out-of-scope
// note).
import { useState } from "react";
import { Link, useLocation } from "react-router-dom";
import ThemeToggle from "./ThemeToggle";
import { useSession, signOut } from "~/lib/auth-client";
import { Menu, X, Gauge } from "lucide-react";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
} from "~/components/ui/popover";

const NAV_LINKS: ReadonlyArray<{ href: string; label: string }> = [
  { href: "/usage", label: "Usage" },
];

/** Return up to two uppercase initials from a display name. */
function initials(name: string): string {
  const parts = name.trim().split(/\s+/);
  if (parts.length === 1) return (parts[0]?.[0] ?? "").toUpperCase();
  return ((parts[0]?.[0] ?? "") + (parts[parts.length - 1]?.[0] ?? "")).toUpperCase();
}

export default function Header() {
  const { data: session, isPending } = useSession();
  const pathname = useLocation().pathname;
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [userPopoverOpen, setUserPopoverOpen] = useState(false);

  async function handleSignOut() {
    await signOut();
    window.location.href = "/";
  }

  function isActive(href: string) {
    if (href === "/") return pathname === "/";
    return pathname.startsWith(href);
  }

  return (
    <header className="sticky top-0 z-50 border-b border-[var(--line)] bg-[var(--header-bg)] px-[clamp(16px,4vw,48px)]">
      {/* story-2/ticket-17: มายด์'s own report — the navbar's content used
          to sit inside a centered max-w-7xl container while the usage
          console body below it (usage-console.css's .usage-console) has
          no max-width at all, stretching edge to edge. On a wide screen
          that put the nav's logo/links visibly narrower than the body
          underneath it. Dropped the cap entirely and matched the header's
          own horizontal padding to the exact clamp() the console body
          already uses, so both line up at any viewport width instead of
          just both being "full width" independently. */}
      <nav className="flex items-center gap-3 py-3 sm:py-4">
        {/* Logo pill */}
        <Link
          to="/"
          className="inline-flex items-center gap-2 rounded-full border border-[var(--chip-line)] bg-[var(--chip-bg)] px-3 py-1.5 text-sm font-semibold text-[var(--sea-ink)] no-underline shadow-[0_8px_24px_rgba(36,29,18,0.08)] sm:px-4 sm:py-2"
        >
          <Gauge className="size-4" />
          {/* Hide text on mobile, show on md+ */}
          <span className="hidden lg:inline">My Token</span>
        </Link>

        {/* Desktop nav links */}
        <div className="hidden items-center gap-1 lg:flex">
          {NAV_LINKS.map((link) => (
            <Link
              key={link.href}
              to={link.href}
              className={[
                "rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
                isActive(link.href)
                  ? "bg-[var(--link-bg-hover)] text-[var(--sea-ink)]"
                  : "text-[var(--sea-ink-soft)] hover:bg-[var(--link-bg-hover)] hover:text-[var(--sea-ink)]",
              ].join(" ")}
            >
              {link.label}
            </Link>
          ))}
        </div>

        <div className="ml-auto flex items-center gap-2">
          <ThemeToggle />

          {/* Desktop: user avatar popover with Settings + Sign out */}
          {!isPending && session && (
            <Popover open={userPopoverOpen} onOpenChange={setUserPopoverOpen}>
              <PopoverTrigger asChild>
                <button
                  className="hidden lg:inline-flex items-center justify-center size-8 rounded-full border border-[var(--line)] bg-[var(--chip-bg)] text-xs font-semibold text-[var(--sea-ink)] transition hover:border-[var(--lagoon)] hover:bg-[var(--link-bg-hover)]"
                  aria-label="User menu"
                  title={session.user.name}
                >
                  {initials(session.user.name)}
                </button>
              </PopoverTrigger>
              <PopoverContent
                align="end"
                className="w-48 p-1 border-[var(--line)] bg-[var(--chip-bg)]"
              >
                <div className="mb-1 px-3 py-2 border-b border-[var(--line)]">
                  <p className="text-xs font-medium text-[var(--sea-ink)] truncate">
                    {session.user.name}
                  </p>
                  <p className="text-xs text-[var(--sea-ink-soft)] truncate">
                    {session.user.email}
                  </p>
                </div>
                <Link
                  to="/settings"
                  onClick={() => setUserPopoverOpen(false)}
                  className={[
                    "block rounded-md px-3 py-2 text-sm font-medium text-[var(--sea-ink)] transition-colors hover:bg-[var(--link-bg-hover)]",
                    pathname === "/settings" ? "bg-[var(--link-bg-hover)]" : "",
                  ].join(" ")}
                >
                  Settings
                </Link>
                <button
                  onClick={() => { setUserPopoverOpen(false); void handleSignOut(); }}
                  className="w-full rounded-md px-3 py-2 text-left text-sm font-medium text-[var(--sea-ink)] transition-colors hover:bg-[var(--link-bg-hover)]"
                >
                  Sign out
                </button>
              </PopoverContent>
            </Popover>
          )}

          {/* Mobile hamburger — visible below lg */}
          <button
            onClick={() => setMobileMenuOpen((prev) => !prev)}
            className="inline-flex size-8 items-center justify-center rounded-full text-[var(--sea-ink-soft)] transition hover:bg-[var(--link-bg-hover)] hover:text-[var(--sea-ink)] lg:hidden"
            aria-label={mobileMenuOpen ? "Close menu" : "Open menu"}
          >
            {mobileMenuOpen ? <X className="size-4" /> : <Menu className="size-4" />}
          </button>
        </div>
      </nav>

      {/* Mobile dropdown — nav links + Settings + Sign out */}
      {mobileMenuOpen && (
        <div className="border-t border-[var(--line)] py-2 lg:hidden">
          {NAV_LINKS.map((link) => (
            <Link
              key={link.href}
              to={link.href}
              onClick={() => setMobileMenuOpen(false)}
              className={[
                "block px-4 py-2.5 text-sm font-medium transition-colors",
                isActive(link.href)
                  ? "bg-[var(--link-bg-hover)] text-[var(--sea-ink)]"
                  : "text-[var(--sea-ink-soft)] hover:bg-[var(--link-bg-hover)] hover:text-[var(--sea-ink)]",
              ].join(" ")}
            >
              {link.label}
            </Link>
          ))}
          {!isPending && session && (
            <>
              <div className="my-2 border-t border-[var(--line)]" />
              <Link
                to="/settings"
                onClick={() => setMobileMenuOpen(false)}
                className={[
                  "block px-4 py-2.5 text-sm font-medium transition-colors",
                  pathname === "/settings"
                    ? "bg-[var(--link-bg-hover)] text-[var(--sea-ink)]"
                    : "text-[var(--sea-ink-soft)] hover:bg-[var(--link-bg-hover)] hover:text-[var(--sea-ink)]",
                ].join(" ")}
              >
                Settings
              </Link>
              <button
                onClick={() => { setMobileMenuOpen(false); void handleSignOut(); }}
                className="block w-full px-4 py-2.5 text-left text-sm font-medium text-[var(--sea-ink-soft)] transition-colors hover:bg-[var(--link-bg-hover)] hover:text-[var(--sea-ink)]"
              >
                Sign out
              </button>
            </>
          )}
        </div>
      )}
    </header>
  );
}

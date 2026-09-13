// TPL-1 milestone-3: react-router wiring. Originally two routes, both
// authenticated and wrapped in one persistent AppLayout so AuthGate/
// Header/Footer don't remount switching between them: "/" and "/settings".
// There is deliberately no SPA route for "/login" — vite.config.ts's proxy
// config (dev) and cmd/server's wireBFF (production) both route a real
// browser navigation to /login straight to Go's own GET /login handler
// before it ever reaches react-router, so a client-side "/login" route
// here would be dead code. See vite.config.ts's proxy comment for why
// that's safe to rely on. task-1 built routing against placeholder page
// content; task-3 wires the real pages underneath the same route table.
//
// story-1/ticket-16 deleted the example domain module's own routes
// ("/{id}", "/activity") along with the domain itself, and pointed
// "/" at UsagePage — the usage console (story-1/ticket-14) is this app's
// only real remaining screen, so both "/" and "/usage" serve it.
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { Toaster } from "~/components/ui/sonner";
import AppLayout from "~/app/AppLayout";
import SettingsPage from "~/app/settings/page";
import UsagePage from "~/app/usage/UsagePage";
import MachinesPage from "~/app/machines/MachinesPage";
import MachineDetailPage from "~/app/machines/MachineDetailPage";

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route
          path="/*"
          element={
            <AppLayout>
              <Routes>
                <Route path="/" element={<UsagePage />} />
                <Route path="/usage" element={<UsagePage />} />
                {/* story-3/ticket-3: the Machines overview page. */}
                <Route path="/machines" element={<MachinesPage />} />
                {/* story-3/ticket-4: the machine detail page this overview
                    page's own row click navigates to. */}
                <Route path="/machines/:installId" element={<MachineDetailPage />} />
                <Route path="/settings" element={<SettingsPage />} />
              </Routes>
            </AppLayout>
          }
        />
      </Routes>
      <Toaster />
    </BrowserRouter>
  );
}

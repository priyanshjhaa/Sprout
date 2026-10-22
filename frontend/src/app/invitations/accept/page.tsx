import { AuthShell } from "@/components/auth/auth-shell";
import { InvitationAccept } from "@/components/access/invitation-accept";

export default function InvitationPage() {
  return <AuthShell mode="invitation"><InvitationAccept /></AuthShell>;
}

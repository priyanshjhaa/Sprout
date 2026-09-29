import { AuthExperience } from "@/components/auth/auth-experience";
import { InvitationAccept } from "@/components/access/invitation-accept";

export default function InvitationPage() {
  return <AuthExperience mode="sign-in"><InvitationAccept /></AuthExperience>;
}

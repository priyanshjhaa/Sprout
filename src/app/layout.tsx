import type { Metadata } from "next";
import { Lora } from "next/font/google";
import type { ReactNode } from "react";
import { Providers } from "@/components/providers";
import "./globals.css";

const heroFont = Lora({
  subsets: ["latin"],
  weight: ["400", "500"],
  style: ["normal", "italic"],
  display: "swap",
  variable: "--font-hero",
});

export const metadata: Metadata = {
  title: "Sprout — small software, ready to share",
  description: "Deploy and share purpose-built software without managing the cloud around it.",
};

export default function RootLayout({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="en" data-scroll-behavior="smooth" className={heroFont.variable}>
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}

import { HomeHero } from "@/components/sections/HomeHero";
import { HomeFeatures } from "@/components/sections/HomeFeatures";
import { HomeIntegrations } from "@/components/sections/HomeIntegrations";
import { HomeCta } from "@/components/sections/HomeCta";

export default function Home() {
  return (
    <main className="flex-1">
      <HomeHero />
      <HomeFeatures />
      <HomeIntegrations />
      <HomeCta />
    </main>
  );
}

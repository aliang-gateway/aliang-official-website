import { HomeHero } from "@/components/sections/HomeHero";
import { HomeFeatures } from "@/components/sections/HomeFeatures";
import { HomeIntegrations } from "@/components/sections/HomeIntegrations";
import { HomeCta } from "@/components/sections/HomeCta";

export default function CompactHomePage() {
  return (
    <>
      <HomeHero variant="compact" />
      <HomeFeatures variant="compact" />
      <HomeIntegrations variant="compact" />
      <HomeCta variant="compact" />
    </>
  );
}

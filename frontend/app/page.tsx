import { HomeHero } from "@/components/sections/HomeHero";
import { HomeFeatures } from "@/components/sections/HomeFeatures";
import { HomeApp } from "@/components/sections/HomeApp";
import { HomeIntegrations } from "@/components/sections/HomeIntegrations";
import { HomeCta } from "@/components/sections/HomeCta";

export default function Home() {
  return (
    <>
      <HomeHero />
      <HomeFeatures />
      <HomeApp />
      <HomeIntegrations />
      <HomeCta />
    </>
  );
}

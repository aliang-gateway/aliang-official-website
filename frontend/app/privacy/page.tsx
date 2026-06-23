"use client";

import { useTranslations } from "next-intl";
import { ContentPage } from "@/components/content/ContentPage";
import { ContentSection, type ContentSectionData } from "@/components/content/ContentSection";

export default function PrivacyPage() {
  const t = useTranslations("privacyPage");
  const sections = t.raw("sections") as ContentSectionData[];

  return (
    <ContentPage
      badge={t("badge")}
      title={t("title")}
      subtitle={t("subtitle")}
      lastUpdated={t("lastUpdated")}
    >
      {sections.map((section, index) => (
        <ContentSection key={index} {...section} />
      ))}
    </ContentPage>
  );
}

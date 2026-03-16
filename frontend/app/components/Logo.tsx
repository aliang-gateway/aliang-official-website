"use client";

import Image from "next/image";
import Link from "next/link";

interface LogoProps {
  showText?: boolean;
  size?: "sm" | "md" | "lg";
}

export default function Logo({ showText = true, size = "md" }: LogoProps) {
  const sizeClasses = {
    sm: { image: 28, text: "text-base" },
    md: { image: 36, text: "text-lg" },
    lg: { image: 44, text: "text-xl" },
  };

  const currentSize = sizeClasses[size];

  return (
    <Link href="/" className="logo-container">
      <div className="logo-icon-wrapper">
        <Image
          src="/logo.svg"
          alt="AI API Portal"
          width={currentSize.image}
          height={currentSize.image}
          className="logo-image"
          priority
        />
      </div>
      {showText && (
        <span className={`logo-text ${currentSize.text}`}>
          <span className="gradient-text">AI API Portal</span>
        </span>
      )}
    </Link>
  );
}

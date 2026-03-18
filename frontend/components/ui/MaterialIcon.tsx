import { cn } from "@/lib/utils";

interface MaterialIconProps {
  name: string;
  size?: number;
  className?: string;
}

export function MaterialIcon({ name, size = 24, className }: MaterialIconProps) {
  return (
    <span
      className={cn("material-symbols-outlined", className)}
      style={{ fontSize: size }}
      aria-hidden="true"
    >
      {name}
    </span>
  );
}

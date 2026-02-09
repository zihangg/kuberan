"use client";

import { cn } from "@/lib/utils";

export const PRESET_COLORS = [
  "#EF4444", // red
  "#F97316", // orange
  "#F59E0B", // amber
  "#EAB308", // yellow
  "#84CC16", // lime
  "#22C55E", // green
  "#10B981", // emerald
  "#14B8A6", // teal
  "#06B6D4", // cyan
  "#3B82F6", // blue
  "#6366F1", // indigo
  "#8B5CF6", // violet
  "#A855F7", // purple
  "#EC4899", // pink
  "#F43F5E", // rose
  "#6B7280", // gray
];

interface ColorPaletteProps {
  value: string;
  onChange: (color: string) => void;
  disabled?: boolean;
}

export function ColorPalette({ value, onChange, disabled }: ColorPaletteProps) {
  return (
    <div className="flex flex-wrap gap-2">
      {PRESET_COLORS.map((color) => (
        <button
          key={color}
          type="button"
          disabled={disabled}
          onClick={() => onChange(value === color ? "" : color)}
          className={cn(
            "h-7 w-7 rounded-full border-2 transition-all",
            value === color
              ? "ring-2 ring-offset-2 ring-primary border-primary"
              : "border-transparent hover:scale-110",
            disabled && "cursor-not-allowed opacity-50"
          )}
          style={{ backgroundColor: color }}
          aria-label={`Select color ${color}`}
        />
      ))}
    </div>
  );
}

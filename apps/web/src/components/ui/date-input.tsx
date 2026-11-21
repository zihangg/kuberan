"use client"

import * as React from "react"
import { CalendarDays } from "lucide-react"

import { cn } from "@/lib/utils"
import {
  isoToDisplay,
  isoToLocalDate,
  localDateToIso,
  maskDateTyping,
  parseDisplayDate,
} from "@/lib/date-input"
import { Calendar } from "@/components/ui/calendar"
import { Input } from "@/components/ui/input"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"

type DateInputProps = Omit<
  React.ComponentProps<"input">,
  "type" | "value" | "defaultValue" | "onChange"
> & {
  /** ISO `yyyy-mm-dd`, or "" for no date. */
  value: string
  /** Called with ISO `yyyy-mm-dd`, or "" when the field is cleared. */
  onChange: (value: string) => void
}

/**
 * Date field that always reads and writes `dd/mm/yyyy`, whatever the browser
 * or device locale (the native `<input type="date">` follows the browser's
 * language, so en-US browsers showed mm/dd/yyyy). The value in and out is
 * still ISO `yyyy-mm-dd`. Type the date (slashes are added on a number pad)
 * or pick it from the calendar button.
 */
function DateInput({
  value,
  onChange,
  className,
  disabled,
  onBlur,
  onKeyDown,
  placeholder = "dd/mm/yyyy",
  ...props
}: DateInputProps) {
  const [text, setText] = React.useState(() => isoToDisplay(value))
  const [open, setOpen] = React.useState(false)

  // Follow value changes made outside the field (reset, prefill, calendar),
  // but leave the text alone when it already says the same date.
  React.useEffect(() => {
    setText((current) =>
      parseDisplayDate(current) === value ? current : isoToDisplay(value)
    )
  }, [value])

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const next = maskDateTyping(e.target.value)
    setText(next)
    // Commit as soon as the text is a whole, real date, so a form submitted
    // with Enter (before blur) already has it.
    const iso = parseDisplayDate(next)
    if (iso !== null && iso !== value) onChange(iso)
  }

  // On leaving the field: an emptied field clears the date, a valid date is
  // tidied (1/5/2026 -> 01/05/2026), and anything incomplete or not a real
  // date puts back the last good one.
  const settle = () => {
    if (text.trim() === "") {
      if (value !== "") onChange("")
      return
    }
    const iso = parseDisplayDate(text)
    setText(iso !== null ? isoToDisplay(iso) : isoToDisplay(value))
  }

  const selected = isoToLocalDate(value)

  return (
    <div className={cn("relative w-full min-w-0", className)}>
      <Input
        type="text"
        inputMode="numeric"
        autoComplete="off"
        placeholder={placeholder}
        value={text}
        onChange={handleChange}
        onBlur={(e) => {
          settle()
          onBlur?.(e)
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") settle()
          onKeyDown?.(e)
        }}
        disabled={disabled}
        className="pr-10 tabular-nums"
        {...props}
      />
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <button
            type="button"
            disabled={disabled}
            aria-label="Pick a date from the calendar"
            className="absolute inset-y-0 right-0 flex w-10 items-center justify-center rounded-r-control text-muted-foreground outline-none hover:text-foreground focus-visible:text-foreground disabled:pointer-events-none disabled:opacity-50"
          >
            <CalendarDays className="size-4" />
          </button>
        </PopoverTrigger>
        <PopoverContent className="w-auto p-0" align="end">
          <Calendar
            mode="single"
            selected={selected}
            defaultMonth={selected}
            weekStartsOn={1}
            onSelect={(day) => {
              if (!day) return
              const iso = localDateToIso(day)
              setText(isoToDisplay(iso))
              if (iso !== value) onChange(iso)
              setOpen(false)
            }}
          />
        </PopoverContent>
      </Popover>
    </div>
  )
}

export { DateInput }
export type { DateInputProps }

"use client"

import * as React from "react"
import { XIcon } from "lucide-react"
import { Dialog as DialogPrimitive } from "radix-ui"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"

function Dialog({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Root>) {
  return <DialogPrimitive.Root data-slot="dialog" {...props} />
}

function DialogTrigger({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Trigger>) {
  return <DialogPrimitive.Trigger data-slot="dialog-trigger" {...props} />
}

function DialogPortal({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Portal>) {
  return <DialogPrimitive.Portal data-slot="dialog-portal" {...props} />
}

function DialogClose({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Close>) {
  return <DialogPrimitive.Close data-slot="dialog-close" {...props} />
}

function DialogOverlay({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Overlay>) {
  return (
    <DialogPrimitive.Overlay
      data-slot="dialog-overlay"
      className={cn(
        "data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 fixed inset-0 z-50 bg-black/50",
        className
      )}
      {...props}
    />
  )
}

function DialogContent({
  className,
  children,
  showCloseButton = true,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Content> & {
  showCloseButton?: boolean
}) {
  return (
    <DialogPortal data-slot="dialog-portal">
      <DialogOverlay />
      <DialogPrimitive.Content
        data-slot="dialog-content"
        className={cn(
          "bg-background scrollbar-slim fixed z-50 flex flex-col gap-4 border shadow-xl outline-none duration-200",
          "data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0",
          // Mobile: bottom sheet that slides up, full width, scrollable. It
          // sits on top of the on-screen keyboard (see KeyboardInset), and the
          // scroll padding keeps a focused field clear of the sticky footer.
          "inset-x-0 bottom-[var(--keyboard-inset,0px)] max-h-[calc(var(--visual-height,100dvh)*0.92)] w-full overflow-y-auto scroll-pb-24 rounded-t-2xl rounded-b-none border-b-0 px-5 pt-3 pb-[calc(1.25rem+env(safe-area-inset-bottom))]",
          "data-[state=closed]:slide-out-to-bottom data-[state=open]:slide-in-from-bottom",
          // Desktop: centered modal that fades/zooms in
          "sm:inset-x-auto sm:top-1/2 sm:bottom-auto sm:scroll-pb-0 sm:left-1/2 sm:max-h-[85vh] sm:max-w-lg sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-xl sm:border-b sm:p-6",
          "sm:data-[state=closed]:slide-out-to-bottom-0 sm:data-[state=open]:slide-in-from-bottom-0 sm:data-[state=closed]:zoom-out-95 sm:data-[state=open]:zoom-in-95",
          className
        )}
        {...props}
      >
        <KeyboardInset />
        {/* Grab handle (mobile bottom-sheet only) */}
        <div
          aria-hidden
          className="mx-auto -mt-1 mb-1 h-1.5 w-10 shrink-0 rounded-full bg-muted-foreground/25 sm:hidden"
        />
        {children}
        {showCloseButton && (
          <DialogPrimitive.Close
            data-slot="dialog-close"
            className="ring-offset-background focus:ring-ring data-[state=open]:bg-accent data-[state=open]:text-muted-foreground absolute top-2 right-2 rounded-md p-3.5 sm:top-4 sm:right-4 sm:p-0.5 opacity-70 transition-opacity hover:opacity-100 focus:ring-2 focus:ring-offset-2 focus:outline-hidden disabled:pointer-events-none [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"
          >
            <XIcon />
            <span className="sr-only">Close</span>
          </DialogPrimitive.Close>
        )}
      </DialogPrimitive.Content>
    </DialogPortal>
  )
}

/**
 * Keeps the mobile bottom sheet above the on-screen keyboard. Android Chrome
 * resizes the layout viewport itself (see `interactiveWidget` in the root
 * layout), but iOS Safari only shrinks the visual viewport, leaving a
 * bottom-anchored sheet — and its submit button — behind the keyboard. This
 * publishes the covered height and the visible height as CSS variables on the
 * sheet, which its mobile classes read. Mounted only while the sheet is open.
 */
function KeyboardInset() {
  const ref = React.useRef<HTMLSpanElement>(null)

  React.useEffect(() => {
    const vv = window.visualViewport
    const sheet = ref.current?.parentElement
    if (!vv || !sheet) return

    function update() {
      if (!vv || !sheet) return
      const layoutHeight = document.documentElement.clientHeight
      const inset = Math.max(0, layoutHeight - vv.height - vv.offsetTop)
      sheet.style.setProperty("--keyboard-inset", `${Math.round(inset)}px`)
      sheet.style.setProperty("--visual-height", `${Math.round(vv.height)}px`)
      // The sheet just shrank: bring the field being typed into back into view.
      const active = document.activeElement
      if (inset > 0 && active instanceof HTMLElement && sheet.contains(active)) {
        requestAnimationFrame(() => active.scrollIntoView({ block: "nearest" }))
      }
    }

    update()
    vv.addEventListener("resize", update)
    vv.addEventListener("scroll", update)
    return () => {
      vv.removeEventListener("resize", update)
      vv.removeEventListener("scroll", update)
    }
  }, [])

  return <span ref={ref} hidden />
}

function DialogHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-header"
      className={cn("flex flex-col gap-2 text-center sm:text-left", className)}
      {...props}
    />
  )
}

function DialogFooter({
  className,
  showCloseButton = false,
  children,
  ...props
}: React.ComponentProps<"div"> & {
  showCloseButton?: boolean
}) {
  return (
    <div
      data-slot="dialog-footer"
      className={cn(
        "flex flex-col-reverse gap-2 sm:flex-row sm:justify-end",
        // Mobile: pinned to the bottom of the scrolling sheet so the primary
        // action is always reachable. It bleeds over the sheet's side and
        // bottom padding, so it must be the sheet's last content. Sticky offsets
        // are measured inside the sheet's padding, hence the negative bottom.
        "sticky bottom-[calc(-1.25rem-env(safe-area-inset-bottom))] z-10 -mx-5 -mb-[calc(1.25rem+env(safe-area-inset-bottom))] border-t bg-background px-5 pt-3 pb-[calc(1.25rem+env(safe-area-inset-bottom))]",
        "sm:static sm:bottom-auto sm:mx-0 sm:mb-0 sm:border-t-0 sm:bg-transparent sm:p-0",
        className
      )}
      {...props}
    >
      {children}
      {showCloseButton && (
        <DialogPrimitive.Close asChild>
          <Button variant="outline">Close</Button>
        </DialogPrimitive.Close>
      )}
    </div>
  )
}

function DialogTitle({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Title>) {
  return (
    <DialogPrimitive.Title
      data-slot="dialog-title"
      className={cn("text-lg leading-none font-semibold", className)}
      {...props}
    />
  )
}

function DialogDescription({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Description>) {
  return (
    <DialogPrimitive.Description
      data-slot="dialog-description"
      className={cn("text-muted-foreground text-sm", className)}
      {...props}
    />
  )
}

export {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
}

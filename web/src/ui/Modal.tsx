import type { ComponentChildren } from 'preact'

interface ModalProps {
  /** Dialog title; callers append any "— <context>" suffix themselves. */
  title: string
  /** Backdrop click and the × button both invoke this. */
  onClose: () => void
  /** Body content (usually settings-field groups). */
  children: ComponentChildren
  /** Optional footer (usually a ghost cancel + primary confirm pair). */
  footer?: ComponentChildren
  /** Wide variant for table-shaped dialogs (login history). */
  wide?: boolean
}

// Shared modal shell (P2-2 dedup). Five admin dialogs (user form, reset
// password, review reject, banned send-back, login history) carried this
// markup verbatim; the backdrop click closes while the panel click stops
// propagation so interacting inside never dismisses the dialog.
export function Modal({ title, onClose, children, footer, wide }: ModalProps) {
  return (
    <div class="modal-backdrop" onClick={onClose}>
      <div class={wide ? 'modal modal--wide' : 'modal'} onClick={(e) => e.stopPropagation()}>
        <div class="modal-head">
          <h3>{title}</h3>
          <button class="btn btn-ghost btn-sm" onClick={onClose}>×</button>
        </div>
        <div class="modal-body">{children}</div>
        {footer && <div class="modal-foot">{footer}</div>}
      </div>
    </div>
  )
}

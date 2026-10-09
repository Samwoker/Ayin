import { Dialog as DialogPrimitive } from 'bits-ui';
import Content from './dialog-content.svelte';
import Description from './dialog-description.svelte';
import Footer from './dialog-footer.svelte';
import Header from './dialog-header.svelte';
import Overlay from './dialog-overlay.svelte';
import Title from './dialog-title.svelte';

const Root = DialogPrimitive.Root;
const Trigger = DialogPrimitive.Trigger;
const Close = DialogPrimitive.Close;
const Portal = DialogPrimitive.Portal;

export {
	Root,
	Title,
	Close,
	Portal,
	Header,
	Footer,
	Overlay,
	Content,
	Trigger,
	Description,
	// Aliases
	Root as Dialog,
	Title as DialogTitle,
	Close as DialogClose,
	Portal as DialogPortal,
	Header as DialogHeader,
	Footer as DialogFooter,
	Overlay as DialogOverlay,
	Content as DialogContent,
	Trigger as DialogTrigger,
	Description as DialogDescription
};

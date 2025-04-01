// clickOutside.ts - A Svelte directive to detect clicks outside an element
import type { ActionReturn } from 'svelte/action';

/**
 * Dispatches a custom "clickoutside" event when a click occurs outside the element.
 * Usage: <div use:clickOutside on:clickoutside={handleClickOutside}>
 */
export function clickOutside(node: HTMLElement): ActionReturn {
  const handleClick = (event: MouseEvent) => {
    if (node && !node.contains(event.target as Node) && !event.defaultPrevented) {
      node.dispatchEvent(new CustomEvent('clickoutside'));
    }
  };
  
  document.addEventListener('click', handleClick, true);
  
  return {
    destroy() {
      document.removeEventListener('click', handleClick, true);
    }
  };
}
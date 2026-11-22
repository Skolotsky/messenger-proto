import { forwardRef, useImperativeHandle, useLayoutEffect, useRef, type TextareaHTMLAttributes } from 'react';

/** A box that holds text is as tall as its text, with no scrollbar until its limit (D-024). */
export const AutoTextarea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(
  function AutoTextarea(props, ref) {
    const box = useRef<HTMLTextAreaElement>(null);
    useImperativeHandle(ref, () => box.current!);
    useLayoutEffect(() => {
      const el = box.current;
      if (!el) return;
      el.style.height = 'auto';
      el.style.height = `${el.scrollHeight}px`;
    }, [props.value]);
    return <textarea rows={1} autoComplete="off" {...props} ref={box} />;
  },
);

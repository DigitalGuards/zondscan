import { useEffect, useRef, useState } from "react";
import type { ChangeEvent, InputHTMLAttributes } from "react";

interface DebouncedInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'onChange'> {
  value: string;
  onChange: (value: string) => void;
  debounce?: number;
  'aria-label'?: string;
}

export default function DebouncedInput({
  value: initialValue,
  onChange,
  debounce = 0,
  'aria-label': ariaLabel = 'Filter',
  ...props
}: DebouncedInputProps): JSX.Element {
  const [value, setValue] = useState<string>(initialValue);
  // React's "adjusting state on prop change" pattern: detect the parent
  // pushing a new initialValue mid-render and resync without scheduling a
  // post-render setState in an effect. Avoids the set-state-in-effect rule
  // and the cascading-render perf footgun.
  const [prevInitial, setPrevInitial] = useState<string>(initialValue);
  if (prevInitial !== initialValue) {
    setPrevInitial(initialValue);
    setValue(initialValue);
  }

  // Keep the latest onChange out of the effect deps: consumers pass inline
  // closures, and a fresh identity per parent render must not re-arm the
  // timer (it would re-emit the unchanged value, e.g. resetting a consumer's
  // page param on every poll re-render).
  const onChangeRef = useRef(onChange);
  useEffect(() => {
    onChangeRef.current = onChange;
  });

  useEffect(() => {
    // Only emit when the user's edit actually diverges from the parent's
    // value: no mount fire (a deep-linked page param would be wiped by an
    // initial onChange), no echo after the parent adopts the emitted value.
    if (value === initialValue) return;
    const timeout = setTimeout(() => {
      onChangeRef.current(value);
    }, debounce);

    return () => clearTimeout(timeout);
  }, [value, initialValue, debounce]);

  const handleChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setValue(e.target.value);
  };

  return (
    <input
      {...props}
      aria-label={ariaLabel}
      value={value}
      onChange={handleChange}
    />
  );
}

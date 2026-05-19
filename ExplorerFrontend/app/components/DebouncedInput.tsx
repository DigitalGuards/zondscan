import { useEffect, useState } from "react";
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

  useEffect(() => {
    const timeout = setTimeout(() => {
      onChange(value);
    }, debounce);

    return () => clearTimeout(timeout);
  }, [value, onChange, debounce]);

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

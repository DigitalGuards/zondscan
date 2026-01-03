interface Props {
  value: string;
}

export default function Alert({ value }: Props): JSX.Element {
  return (
    <div className="px-5 py-2 text-white bg-red-500 rounded-md">{value}</div>
  );
}
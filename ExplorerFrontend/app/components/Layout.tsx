import type { PropsWithChildren } from "react";
import Header from "./Header";

const Layout = ({ children }: PropsWithChildren): JSX.Element => {
  return (
    <>
      <Header />
      <section>{children}</section>
    </>
  );
};

export default Layout;
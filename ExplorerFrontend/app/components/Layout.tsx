import React, { PropsWithChildren } from "react";
import Header from "./Header";

const Layout = ({ children }: PropsWithChildren) => {
  return (
    <>
      <Header />
      <section>{children}</section>;
    </>
  );
};
export default Layout;
import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { jwtVerify } from "jose";

const PUBLIC_PATHS = [
  "/login",
  "/api/",
  "/_next/",
  "/favicon.ico",
];

function isPublicPath(pathname: string): boolean {
  return PUBLIC_PATHS.some((p) => pathname.startsWith(p));
}

export async function middleware(req: NextRequest) {
  const { pathname } = req.nextUrl;

  if (isPublicPath(pathname)) {
    return NextResponse.next();
  }

  const gatewayUrl = process.env.AUTH_GATEWAY_URL;
  if (!gatewayUrl) {
    return NextResponse.next();
  }

  const jwtSecret = process.env.AUTH_JWT_SECRET;
  if (!jwtSecret) {
    return NextResponse.next();
  }

  const cookieName = process.env.AUTH_COOKIE_NAME || "_claw_auth";
  const token = req.cookies.get(cookieName)?.value;

  if (!token) {
    const loginUrl = `${gatewayUrl}/login?redirect=${encodeURIComponent(req.url)}`;
    return NextResponse.redirect(loginUrl);
  }

  try {
    const secret = new TextEncoder().encode(jwtSecret);
    const { payload } = await jwtVerify(token, secret);

    if (payload.approved === false) {
      const loginUrl = `${gatewayUrl}/login?redirect=${encodeURIComponent(req.url)}`;
      return NextResponse.redirect(loginUrl);
    }

    return NextResponse.next();
  } catch {
    const loginUrl = `${gatewayUrl}/login?redirect=${encodeURIComponent(req.url)}`;
    const response = NextResponse.redirect(loginUrl);
    response.cookies.delete(cookieName);
    return response;
  }
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};

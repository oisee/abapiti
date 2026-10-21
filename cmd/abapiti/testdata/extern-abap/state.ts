import {env} from "./host.ts";

export const greeting: string = "smoke:" + (env("ABAPITI_SMOKE") ?? "unset");

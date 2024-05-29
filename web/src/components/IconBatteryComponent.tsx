import { SVGProps } from "react"

const defaultProps = {
        stroke: "currentColor",
        fill: "currentColor",
        strokeWidth: 0,
        version: "1.2",
        baseProfile: "tiny",
        viewBox: "0 0 24 14",
        height: "24px",
        width: "24px",
        xmlns: "http://www.w3.org/2000/svg",
}



export const BatteryCharge = (props: SVGProps<SVGSVGElement>) => (
    <svg {... props}>
        <path d="M5 10v6h11v-6h-11zm5.83 4.908l-1.21-1.908-2.62.428 3.223-2.324 1.175 1.896 2.602-.43-3.17 2.338zM19 10c0-1.654-1.346-3-3-3h-11c-1.654 0-3 1.346-3 3v6c0 1.654 1.346 3 3 3h11c1.654 0 3-1.346 3-3 1.104 0 2-.896 2-2v-2c0-1.104-.896-2-2-2zm-2 6c0 .552-.449 1-1 1h-11c-.551 0-1-.448-1-1v-6c0-.552.449-1 1-1h11c.551 0 1 .448 1 1v6z"></path>
    </svg>
);
BatteryCharge.defaultProps = defaultProps


export const BatteryLow = (props: SVGProps<SVGSVGElement>) => (
    <svg {... props}>
        <path d="m 7.0000005,10 c -0.552,0 -1,-0.4469995 -1,-0.9999995 v -4 c 0,-0.553 0.448,-1 1,-1 0.552,0 1,0.447 1,1 v 4 c 0,0.553 -0.448,0.9999995 -1,0.9999995 z M 20,4.0000005 c 0,-1.654 -1.346,-3 -3,-3 H 6.0000005 c -1.654,0 -3,1.346 -3,3 V 10 c 0,1.654 1.346,3 3,3 H 17 c 1.654,0 3,-1.346 3,-3 1.104,0 2,-0.8959995 2,-1.9999995 v -2 c 0,-1.104 -0.896,-2 -2,-2 z M 18,10 c 0,0.552 -0.449,1 -1,1 H 6.0000005 c -0.551,0 -1,-0.448 -1,-1 V 4.0000005 c 0,-0.552 0.449,-1 1,-1 H 17 c 0.551,0 1,0.448 1,1 z"/>
    </svg>
);
BatteryLow.defaultProps = defaultProps

export const BatteryMid = (props: SVGProps<SVGSVGElement>) => (
    <svg {... props}>
        <path d="M9 16c-.552 0-1-.447-1-1v-4c0-.553.448-1 1-1s1 .447 1 1v4c0 .553-.448 1-1 1zM6 16c-.552 0-1-.447-1-1v-4c0-.553.448-1 1-1s1 .447 1 1v4c0 .553-.448 1-1 1zM19 10c0-1.654-1.346-3-3-3h-11c-1.654 0-3 1.346-3 3v6c0 1.654 1.346 3 3 3h11c1.654 0 3-1.346 3-3 1.104 0 2-.896 2-2v-2c0-1.104-.896-2-2-2zm-2 6c0 .552-.449 1-1 1h-11c-.551 0-1-.448-1-1v-6c0-.552.449-1 1-1h11c.551 0 1 .448 1 1v6z"></path>
    </svg>
);
BatteryMid.defaultProps = defaultProps

export const BatteryHigh = (props: SVGProps<SVGSVGElement>) => (
    <svg {... props}>
        <path d="M9 16c-.552 0-1-.447-1-1v-4c0-.553.448-1 1-1s1 .447 1 1v4c0 .553-.448 1-1 1zM6 16c-.552 0-1-.447-1-1v-4c0-.553.448-1 1-1s1 .447 1 1v4c0 .553-.448 1-1 1zM12 16c-.552 0-1-.447-1-1v-4c0-.553.448-1 1-1s1 .447 1 1v4c0 .553-.448 1-1 1zM19 10c0-1.654-1.346-3-3-3h-11c-1.654 0-3 1.346-3 3v6c0 1.654 1.346 3 3 3h11c1.654 0 3-1.346 3-3 1.104 0 2-.896 2-2v-2c0-1.104-.896-2-2-2zm-2 6c0 .552-.449 1-1 1h-11c-.551 0-1-.448-1-1v-6c0-.552.449-1 1-1h11c.551 0 1 .448 1 1v6z"></path>
    </svg>
);
BatteryHigh.defaultProps = defaultProps

export const BatteryFull = (props: SVGProps<SVGSVGElement>) => (
    <svg {... props}>
        <path d="M9 16c-.552 0-1-.447-1-1v-4c0-.553.448-1 1-1s1 .447 1 1v4c0 .553-.448 1-1 1zM6 16c-.552 0-1-.447-1-1v-4c0-.553.448-1 1-1s1 .447 1 1v4c0 .553-.448 1-1 1zM15 16c-.552 0-1-.447-1-1v-4c0-.553.448-1 1-1s1 .447 1 1v4c0 .553-.448 1-1 1zM12 16c-.552 0-1-.447-1-1v-4c0-.553.448-1 1-1s1 .447 1 1v4c0 .553-.448 1-1 1zM19 10c0-1.654-1.346-3-3-3h-11c-1.654 0-3 1.346-3 3v6c0 1.654 1.346 3 3 3h11c1.654 0 3-1.346 3-3 1.104 0 2-.896 2-2v-2c0-1.104-.896-2-2-2zm-2 6c0 .552-.449 1-1 1h-11c-.551 0-1-.448-1-1v-6c0-.552.449-1 1-1h11c.551 0 1 .448 1 1v6z"></path>
    </svg>
);
BatteryFull.defaultProps = defaultProps



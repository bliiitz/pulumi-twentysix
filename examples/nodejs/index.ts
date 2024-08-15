import * as pulumi from "@pulumi/pulumi";
import * as twentysix from "@pulumi/twentysix";

const myRandomResource = new twentysix.Random("myRandomResource", {length: 24});
export const output = {
    value: myRandomResource.result,
};

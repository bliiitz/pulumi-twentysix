import pulumi
import pulumi_twentysix as twentysix

my_random_resource = twentysix.Random("myRandomResource", length=24)
pulumi.export("output", {
    "value": my_random_resource.result,
})

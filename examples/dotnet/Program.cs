using System.Collections.Generic;
using System.Linq;
using Pulumi;
using twentysix = Pulumi.twentysix;

return await Deployment.RunAsync(() => 
{
    var myRandomResource = new twentysix.Random("myRandomResource", new()
    {
        Length = 24,
    });

    return new Dictionary<string, object?>
    {
        ["output"] = 
        {
            { "value", myRandomResource.Result },
        },
    };
});


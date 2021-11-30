# About Hardened Images

Hardened images are those that have been scanned for vulnerabilities, and with each new image build, additional security protections are added to decrease potential weaknesses.

The hardened images in RKE2 are not simply mirrored from upstream. The images get built on top of a hardened and minimalized base image, which is currently Universal Base Images (UBI).

For any binaries that are written in Go, they are compiled using a FIPS 140-2 compliant build process. For more information on this compiler, refer [here](https://docs.rke2.io/security/fips_support/#use-of-fips-compatible-go-compiler).

You will know if an image has been hardened as above by the image name. RKE2 publishes image lists with each release. Refer [here](https://github.com/rancher/rke2/releases/download/v1.22.3-rc1%2Brke2r1/rke2-images-all.linux-amd64.txt) for an example of a published image list.

!!! note "Note:" 
Currently, RKE2 hardened images are multi-architecture. Only the Linux AMD64 architecture is FIPS compliant. Windows and the soon-to-come s390x architectures are not FIPS compliant.
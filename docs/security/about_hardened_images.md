# About Hardened Images

RKE2 hardened images are scanned for vulnerabilities at build time, and additional security protections have been added to decrease potential weaknesses:
* Images are not simply mirrored from upstream builds. The images are built from source on top of a hardened minimal base image, which is currently Red Hat Universal Base Image (UBI).
* Any binaries that are written in Go are compiled using a FIPS 140-2 compliant build process. For more information on this compiler, refer [here](https://docs.rke2.io/security/fips_support/#use-of-fips-compatible-go-compiler).

You will know if an image has been hardened as above by the image name. RKE2 publishes image lists with each release. Refer [here](https://github.com/rancher/rke2/releases/download/v1.22.3-rc1%2Brke2r1/rke2-images-all.linux-amd64.txt) for an example of a published image list.

!!! note "Note:" 
Currently, RKE2 hardened images are multi-architecture. Only the Linux AMD64 architecture is FIPS compliant. Windows and the soon-to-come s390x architectures are not FIPS compliant.
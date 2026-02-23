#!/bin/bash

info()
{
    echo '[INFO] ' "$@"
}
warn()
{
    echo '[WARN] ' "$@" >&2
}
fatal()
{
    echo '[ERROR] ' "$@" >&2
    exit 1
}

HELM_REPO="https://rancher.github.io/rke2-charts"

update_chart_version() {
    info "updating chart ${1} in ${CHART_VERSIONS_FILE}"
    CURRENT_VERSION=$(yq -r '.charts[] | select(.filename == "/charts/'"${1}"'.yaml") | .version' ${CHART_VERSIONS_FILE})
    NEW_VERSION=${2}
    if [ "${CURRENT_VERSION}" != "${NEW_VERSION}" ]; then
        info "found version ${CURRENT_VERSION}, updating to ${NEW_VERSION}"
        chart_updated=true
        if test "$DRY_RUN" == "false"; then
            sed -i "s/${CURRENT_VERSION}/${NEW_VERSION}/g" ${CHART_VERSIONS_FILE}
        else
            info "dry-run is enabled, no changes will occur"
        fi
    else
        info "no new version found"
    fi
}

update_chart_images() {
    info "downloading chart ${1} version ${2} to extract image versions"
    tempdir=$(mktemp -d)
    if test "$chart_updated" == "true"; then
        # get all images and tags for the latest constraint
	cni=$(echo "${1}" | sed -nE 's/rke2-(.*)/\1/p')
	case "${1}" in
           "rke2-calico")
              app_version="${2%??}"
	      sed -i "/mirrored-calico-operator/b; /mirrored-calico/s/:v[0-9.]*$/:${app_version}/" ${CHART_AIRGAP_IMAGES_FILE}
	      IMAGES_TAG=$(helm template ${1} --repo ${HELM_REPO} --version ${2} | yq -r '[.. | .image? | select(.)] | .[]' | sort -u)
              ;;
	   "rke2-cilium")
              IMAGES_TAG_AWS=$(helm template ${1} --repo ${HELM_REPO} --version ${2} --set eni.enabled=true |  yq -r '[.. | .image? | select(.)] | .[]' | sort -u)
	      IMAGES_TAG_AZURE=$(helm template ${1} --repo ${HELM_REPO} --version ${2} --set azure.enabled=true |  yq -r '[.. | .image? | select(.)] | .[]' | sort -u)
	      IMAGES_TAG_GENERIC=$(helm template ${1} --repo ${HELM_REPO} --version ${2} -f updatecli/scripts/cilium_values.yaml |  yq -r '[.. | .image? | select(.)] | .[]' | sort -u)
	      IMAGES_TAG=$(echo "${IMAGES_TAG_AWS}"$'\n'"${IMAGES_TAG_AZURE}"$'\n'"${IMAGES_TAG_GENERIC}" | sort -u)
	      ;;
	   "rke2-multus")
	      IMAGES_TAG_THICK=$(helm template ${1} --repo ${HELM_REPO} --version ${2} --set thickPlugin.enabled=true --set dynamicNetworksController.enabled=true | yq -r '[.. | .image? | select(.)] | .[]' | sort -u)
	      IMAGES_TAG_MULTUS=$(helm template ${1} --repo ${HELM_REPO} --version ${2} --set rke2-whereabouts.enabled=true | yq -r '[.. | .image? | select(.)] | .[]' | sort -u)
	      IMAGES_TAG="${IMAGES_TAG_THICK}"$'\n'"${IMAGES_TAG_MULTUS}"
	      ;;
	   *)
	      IMAGES_TAG=$(helm template ${1} --repo ${HELM_REPO} --version ${2} | yq -r '[.. | .image? | select(.)] | .[]' | sort -u)
	      ;;
        esac
	LIST_IMAGES=""
        while IFS= read -r line ; do 
	      IMAGE=$(echo "${line}" | sed 's|.*\/\([^/][^/]*\/[^/][^/]*\)$|\1|; s|/|\\/|g' | tr -dc '[:alnum:]:.\-/\\')
	      LIST_IMAGES=$(echo "$LIST_IMAGES\${REGISTRY}/${IMAGE}\n")
	done <<< "$IMAGES_TAG"
        if test "$DRY_RUN" == "false"; then
	      awk -v images_list="$LIST_IMAGES" -v cni="$cni" '
BEGIN {
    split(images_list, images_array, "\n");
    for (i in images_array) {
        n = split(images_array[i], image_tag, ":");
        if (n == 2) images_list_array[image_tag[1]] = image_tag[2];
    }
    pattern = "xargs.*DIR/images-" cni ".txt.*"
}
$0 ~ pattern {
    print;
    while (getline > 0) {
	if ($1 == "EOF") {
            print;
            next;
        }
        n = split($1, current_image, ":");
        if (n == 2 && current_image[1] in images_list_array) {
            print "    " current_image[1] ":" images_list_array[current_image[1]];
        } else {
            print $0;
        }
    }
}
{ print }' "${CHART_AIRGAP_IMAGES_FILE}" > "$tempdir/images_file.sh"
              cp $tempdir/images_file.sh ${CHART_AIRGAP_IMAGES_FILE}
        else
              info "dry-run is enabled, no changes will occur"
         fi
    else
        info "no new version found"
    fi
    # removing downloaded artifacts
    rm -rf $tempdir/
}

update_chart_images_windows() {
    app_version="${2%??}"
    tempdir=$(mktemp -d)
    case "${1}" in
       "rke2-flannel")
	  flanneld_sha256=$(gh api repos/flannel-io/flannel/releases/tags/${app_version}   --jq '.assets[] | select(.name == "flanneld.exe") | .digest' | cut -d':' -f2)
          sed -i "s/echo \".*  flanneld.exe\" | sha256sum -c -/echo \"${flanneld_sha256}  flanneld.exe\" | sha256sum -c -/g" Dockerfile.windows
	  sed -i "s/ENV FLANNEL_VERSION=.*/ENV FLANNEL_VERSION=\"$app_version\"/g" Dockerfile.windows
	  CNI_VERSION=$(helm template flannel --repo https://flannel-io.github.io/flannel/ --version "$app_version" 2>/dev/null | \
                        grep -oP 'flannel-cni-plugin:\K[\w.-]+' | \
                        head -n 1)
	  if [ -z "$CNI_VERSION" ]; then
             echo "Error: Failed to extract CNI version."
             echo "Check if version $app_version exists at https://flannel-io.github.io/flannel/"
             exit 1
          fi
	  flannel_plugin_sha256=$(gh api repos/flannel-io/cni-plugin/releases/tags/${CNI_VERSION} --jq '.assets[] | select(.name == "flannel-amd64.exe") | .digest' | cut -d':' -f2)
	  sed -i "s/echo \".*  flannel.exe\" | sha256sum -c -/echo \"${flannel_plugin_sha256}  flannel.exe\" | sha256sum -c -/g" Dockerfile.windows
	  sed -i "s/ENV CNI_FLANNEL_VERSION=.*/ENV CNI_FLANNEL_VERSION=\"${CNI_VERSION}\"/g" Dockerfile.windows
	  CLEAN_VERSION=$(helm template rke2-flannel --repo ${HELM_REPO} --version ${2} |  grep -oP 'hardened-cni-plugins:\K[\w.-]+' | head -n 1 | cut -d'-' -f1)
	  cni_plugins_sha256=$(gh api repos/containernetworking/plugins/releases/tags/${CLEAN_VERSION}  --jq '.assets[] | select(.name == "cni-plugins-windows-amd64-'${CLEAN_VERSION}'.tgz") | .digest' | cut -d':' -f2)
	  sed -i "s/echo \".*  cni-plugins-windows-amd64-.*.tgz\" | sha256sum -c -/echo \"${cni_plugins_sha256}  cni-plugins-windows-amd64-\${CNI_PLUGIN_VERSION}.tgz\" | sha256sum -c -/g" Dockerfile.windows
	  sed -i "s/ENV CNI_PLUGIN_VERSION=.*/ENV CNI_PLUGIN_VERSION=\"${CLEAN_VERSION}\"/g" Dockerfile.windows
	  ;;
       "rke2-calico")
	  windows_sha256=$(gh api repos/projectcalico/calico/releases/tags/${app_version}   --jq '.assets[] | select(.name == "calico-windows-'${app_version}'.zip") | .digest' | cut -d':' -f2)
	  sed -i "s/echo \".*  calico-windows-.*.zip\" | sha256sum -c -/echo \"${windows_sha256}  calico-windows-\${CALICO_VERSION}.zip\" | sha256sum -c -/g" Dockerfile.windows
          sed -i "s/ENV CALICO_VERSION=.*/ENV CALICO_VERSION=\"$app_version\"/g" Dockerfile.windows
	  ;;
    esac
    rm -rf $tempdir/
}

CHART_VERSIONS_FILE="charts/chart_versions.yaml"
CHART_AIRGAP_IMAGES_FILE="scripts/build-images"


CHART_NAME=${1}
CHART_VERSION=${2}
chart_updated=false

update_chart_version ${CHART_NAME} ${CHART_VERSION}
update_chart_images ${CHART_NAME} ${CHART_VERSION}
if [ ${CHART_NAME} == "rke2-flannel" -o ${CHART_NAME} == "rke2-calico" ]; then
   update_chart_images_windows ${CHART_NAME} ${CHART_VERSION}
fi
